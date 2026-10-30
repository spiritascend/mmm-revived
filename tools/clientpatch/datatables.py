import argparse
import ctypes
import json
import os
import re
import struct
import sys
from pathlib import Path

sys.dont_write_bytecode = True
sys.path.insert(0, str(Path(__file__).resolve().parent))

from client_modules import LEGACY, REPO, Toolchain, parse

DATATABLES = 'assets/datatables'


def plain(value):
    if isinstance(value, dict):
        keys = list(value)
        if keys and all(isinstance(key, int) for key in keys) and set(keys) == set(range(1, len(keys) + 1)):
            return [plain(value[index]) for index in range(1, len(keys) + 1)]
        return {str(key): plain(child) for key, child in value.items()}
    return value


def sequence(value):
    return value if isinstance(value, list) else []


def mapping(value):
    return {str(index + 1): row for index, row in enumerate(value)} if isinstance(value, list) else value


def ints(value):
    return [item for item in sequence(value) if isinstance(item, int) and not isinstance(item, bool)]


class Tables:
    def __init__(self, tools, analysis, overrides=None):
        self.tools = tools
        self.overrides = overrides or {}
        self.folder = analysis / 'installed'
        self.rows = json.loads((self.folder / 'index.json').read_text())
        self.index = {row['name']: row for row in self.rows}
        self.cache = {}

    def module(self, name):
        if name not in self.cache:
            raw = (self.folder / self.index[name + '.lua']['file']).read_bytes()
            self.cache[name] = {str(key): plain(value) for key, value in self.tools.module_data(raw, name + '.lua').items()}
            for key, fields in self.overrides.get(name, {}).items():
                if isinstance(self.cache[name].get(key), dict):
                    self.cache[name][key].update(fields)
        return self.cache[name]

    def released(self, name):
        versions = set(self.module('version_control_data'))
        return {key for key, row in self.module(name).items() if str(row.get('version')) in versions}

    def code_module(self, name):
        if not hasattr(self, 'shim'):
            lib, state = self.tools.lib, self.tools.state
            function = ctypes.CFUNCTYPE(ctypes.c_int, ctypes.c_void_p)

            def identity(pointer):
                lib.lua_settop(pointer, 1)
                return 1

            self.shim = function(identity)
            lib.lua_getglobal.argtypes = lib.lua_setglobal.argtypes = [ctypes.c_void_p, ctypes.c_char_p]
            lib.lua_pushcclosure.argtypes = [ctypes.c_void_p, function, ctypes.c_int]
            lib.lua_setfield.argtypes = [ctypes.c_void_p, ctypes.c_int, ctypes.c_char_p]
            lib.lua_createtable.argtypes = [ctypes.c_void_p, ctypes.c_int, ctypes.c_int]
            lib.lua_setmetatable.argtypes = [ctypes.c_void_p, ctypes.c_int]
            lib.lua_getglobal(state, b'table')
            lib.lua_pushcclosure(state, self.shim, 0)
            lib.lua_setfield(state, -2, b'const')
            lib.lua_settop(state, 0)
            lib.lua_createtable(state, 0, 0)
            lib.lua_createtable(state, 0, 0)
            lib.lua_createtable(state, 0, 0)
            lib.lua_pushcclosure(state, self.shim, 0)
            lib.lua_setfield(state, -2, b'__call')
            lib.lua_setmetatable(state, -2)
            lib.lua_setfield(state, -2, b'Color')
            lib.lua_setglobal(state, b'u3d')
        return self.module(name)

    def data_tables(self):
        for name in sorted(self.index):
            stem = name[:-4]
            if '_data' in stem and stem != 'task_data':
                try:
                    yield stem, self.module(stem)
                except (RuntimeError, ValueError):
                    continue

    def code_constants(self):
        def walk(node):
            for tag, value in node['constants']:
                if tag == 19:
                    yield struct.unpack('<q', value)[0]
                elif tag in (4, 20) and value and value.isdigit():
                    yield int(value)
            for child in node['children']:
                yield from walk(child)

        for row in self.rows:
            if '_data' not in row['name']:
                yield from walk(parse((self.folder / row['file']).read_bytes())[1])


def scored(t, name):
    for row in t.rows:
        if row['name'] == name + '.lua':
            data = {str(key): plain(value) for key, value in t.tools.module_data((t.folder / row['file']).read_bytes(), row['name']).items()}
            if data and all(isinstance(value, dict) and 'score' in value for value in data.values()):
                return data
    raise ValueError(name)


def skin_collection(t):
    levels = scored(t, 'skin_bonus_collection')
    return {'scores': {key: row['score'] for key, row in scored(t, 'skin_bonus_equip').items()}, 'levels': [levels[str(level)]['score'] for level in range(1, len(levels) + 1)]}


def rules(t):
    load = t.module
    missions, nodes, chapters = load('dreamland_mission_data'), load('dreamland_node_data'), load('dreamland_chapter_data')
    items, heroes, artifacts, highlights = load('item_data_cn'), load('character_data_new_cn'), load('artifact_data_cn'), load('highlight_data_cn')
    team_breaks, activities, pvp, shops = load('team_break_data'), load('activity_data'), load('realtimePVP_data'), load('shop_data_cn')
    item_rules = {}
    for key, row in items.items():
        param = row.get('param') or []
        first = param[0] if isinstance(param, list) and param else 0
        second = param[1] if isinstance(param, list) and len(param) > 1 else 0
        item_rules[key] = {'type': row['type'], 'rarity': row.get('rarity', 0), 'parameter': first if isinstance(first, int) else 0, 'secondary': second if isinstance(second, int) else 0, 'refund': second if isinstance(second, dict) else {}, 'choices': first if isinstance(first, list) else [], 'amounts': second if isinstance(second, list) else []}
    memento_maps = {}
    for node in nodes.values():
        if node.get('nodeType') != 5:
            continue
        tiles = {}
        for tile in load(str(node['param']))['NodeList']:
            parameter = tile.get('EventParam', '')
            tiles[str(tile['Id'])] = {'type': tile['EventType'], 'initial': tile['Purified'], 'next': tile.get('NextNodes') or [], 'moon': tile.get('InitMoon', 0), 'camp': tile.get('Camp', 0), 'param': int(parameter) if parameter.lstrip('-').isdigit() else 0, 'targets': [int(value) for value in parameter.split(',') if value] if tile['EventType'] == 6 else []}
        memento_maps[str(node['param'])] = tiles
    chapter_tags = {key: row.get('tag', '') for key, row in chapters.items()}
    bond_levels = sorted(load('bond_level_data_cn').values(), key=lambda row: (row['bonds_type'], row.get('lv', 0)))
    return {
        'item_rules': item_rules,
        'dispatch_missions': {key: {'heroes': row['number'], 'camps': row.get('camp') or [], 'hero_level': row.get('characterLevel', 0), 'team_level': row.get('teamLevel', 0), 'maximum_level': row.get('teamLevelMax', 0), 'reward': row.get('reward', 0), 'extra_reward': row.get('extraReward', 0)} for key, row in load('dispatch_mission_data').items()},
        'dispatch_maps': sorted(int(key) for key in load('dispatch_map_data_cn')),
        'dispatch_limits': {key: row.get('dispatches', 0) for key, row in team_breaks.items()},
        'artifact_templates': {key: {'attributes': row.get('sub_attr') or {}, 'weights': row.get('sub_weight') or {}} for key, row in load('artifact_temp_data').items()},
        'artifact_salvage': {key: mapping(row) for key, row in load('artifact_dec_data').items()},
        'highlight_salvage': {key: {level: mapping(costs) for level, costs in mapping(row).items()} for key, row in load('highlight_dec_data').items()},
        'highlight_awake_salvage': load('highlight_awake_dec_data'),
        'proficiency_levels': {key: row.get('exp', 0) for key, row in load('hero_prof_lv_data').items()},
        'proficiency_rewards': {key: {str(level): row.get('level' + str(level), {}).get('reward', 0) for level in range(1, 6)} for key, row in load('hero_prof_rwd_data').items()},
        'crafts': {key: {'type': row.get('type', ''), 'initial': row.get('init', 0), 'materials': row.get('necessary_material') or {}, 'optional': row.get('optional_material') or [], 'optional_count': row.get('optional_material_num', 0), 'gold': row.get('gold', 0), 'reward': row.get('product', 0), 'banned_highlights': row.get('ban_highlight') or []} for key, row in load('craft_data').items()},
        'shops': {key: {'currency': row.get('currency_type', 0), 'prices': row.get('price') or [], 'reward': row.get('reward', 0), 'level': row.get('unlock_lv', 1), 'limit': row.get('limit_param', 0), 'period': row.get('limit_type', 0), 'start': row.get('start_time', 0), 'end': row.get('end_time', 0), 'guild_level': row.get('alliance_lv', 0), 'recharge': row.get('recharge_id', 0), 'pre_id': row.get('pre_id', 0), 'pre_task': row.get('pre_task', 0), 'period_time': row.get('period_time', 0)} for key, row in shops.items()},
        'missions': {key: {'chapter': row.get('chapter', 0), 'memento': chapter_tags.get(str(row.get('chapter', 0)), '').startswith('memento'), 'first_node': row.get('firstInitNode', 0), 'max_steps': row.get('maxRound', 0), 'deployment_level': row.get('deploymentLv', 0), 'switches': mapping(row.get('switches', {})), 'star_conditions': row.get('starNeed') or [], 'boss_penalty': row.get('nerfBossLevel', 0), 'energy': row.get('energyCost', 0), 'level': row.get('unlockLv', 1), 'prerequisites': row.get('befMission') or [], 'reward': row.get('firstReward', 0)} for key, row in missions.items()},
        'nodes': {key: {'next': row.get('nextNode', 0), 'type': row.get('nodeType', 0), 'battle': row.get('param', 0) if row.get('nodeType') == 1 else 0, 'map': row.get('param', 0) if row.get('nodeType') == 5 else 0} for key, row in nodes.items()},
        'memento_maps': memento_maps,
        'levels': {key: {'break_level': row.get('lv2break', 0), 'experience': row.get('exp', 0), 'money': row.get('money', 0)} for key, row in load('character_level_data').items()},
        'breaks': {key: {str(index + 1): ({str(position): item for position, item in enumerate(cost, 1)} if isinstance(cost, list) else cost) for index, cost in enumerate(value)} if isinstance(value, list) else value for key, value in load('character_break_data').items()},
        'hero_break_sets': {key: row.get('break_set', 0) for key, row in heroes.items()},
        'rewards': {key: [value[:2] for value in sequence(row.get('preview')) if isinstance(value, list) and len(value) >= 2 and isinstance(value[1], (int, float)) and not (len(value) > 2 and all(items.get(str(entry), {}).get('type') == 1002 for entry in value))] for key, row in load('reward_data').items()},
        'drop_details': {key: {str(entry[0]): entry[1] for entry in sequence(row.get('dropdetail')) if isinstance(entry, list) and len(entry) >= 2} for key, row in load('reward_details_data').items()},
        'tasks': tasks(t),
        'battle_settings': {key: {'heroes': row.get('heroNum', 3), 'fixed_heroes': row.get('hero') or [], 'trial_heroes': row.get('trialHero') or [], 'cards': row.get('commonCard') or [], 'card_count': row.get('commonCardNum', 0)} for key, row in load('battle_mode_new_data_cn').items()},
        'battle_modes': {key: row.get('group', 0) for key, row in load('battle_mode_new_data_cn').items()},
        'team_levels': {key: {'break': row.get('break_tag', 0), 'experience': row.get('exp', 0), 'hero_limit': row.get('hero_level_limit', 0), 'reward': row.get('rewardId', 0), 'energy_limit': row.get('energy_limit', 0), 'energy_recover': row.get('energy_recover', 0)} for key, row in load('player_level_data').items()},
        'highlight_levels': {key: {'break_level': row.get('break_need', 0), 'money': row.get('exp', 0)} for key, row in load('highlight_level_data').items()},
        'highlight_breaks': {key: {tier: mapping(row.get('item', {})) for tier, row in mapping(value).items()} for key, value in load('highlight_break_data').items()},
        'highlight_templates': {key: {'rarity': row.get('rarity', 0), 'break_set': row.get('break_set', 0), 'awake_item': row.get('awake_item', 0), 'classified': 'classifyMark' in row, 'career': row.get('career', 0), 'item': row.get('itemId', 0), 'unreleased': key not in t.released('highlight_data_cn')} for key, row in highlights.items()},
        'skins': {key: row.get('owner', 0) for key, row in load('skin_res_data_cn').items()},
        'skin_rarities': {key: row.get('rarity', 0) for key, row in load('skin_res_data_cn').items()},
        'skin_collection': skin_collection(t),
        'artifacts': {key: {'refinable': row.get('isrefresh') == 1, 'sub_levels': row.get('sub_attr_lv') or [], 'position': row.get('position', 0), 'max_level': row.get('max_lv', 0), 'star': row.get('star', 0), 'template': row.get('template') or [], 'pool': row.get('pool') or [], 'unique': row.get('isrefresh') == 1, 'item': row.get('itemId', 0), 'unreleased': key not in t.released('artifact_data_cn')} for key, row in artifacts.items()},
        'artifact_levels': {key: {str(index + 1): {'experience': row.get('exp_need', 0), 'money': row.get('money', 0)} for index, row in enumerate(value)} if isinstance(value, list) else value for key, value in load('artifact_up_data').items()},
        'hero_stars': {key: row.get('item', {}) for key, row in load('character_star_data').items()},
        'hero_talents': {key: {'break': row.get('break_need', 0), 'cost': row.get('item') or {}} for key, row in load('character_talent_data_cn').items()},
        'hero_talent_sets': {key: row['talent_set'] for key, row in heroes.items() if row.get('talent_set')},
        'hero_awake': {key: {'prerequisites': row.get('pre_id') or [], 'cost': row.get('item') or []} for key, row in load('character_awake_data_cn').items()},
        'energy': load('buy_energy_data'),
        'chapters': {key: {'missions': row.get('mission') or [], 'rewards': row.get('missionReward') or {}} for key, row in chapters.items()},
        'pvp_rounds': load('realtimePVP_round_data'),
        'pvp_battle_maps': {key: sequence(row.get('battleMap'))[0] for key, row in load('real_time_pvp_battle_data').items() if sequence(row.get('battleMap'))},
        'hero_traits': {key: {'career': row.get('career', 0), 'camp': row.get('camp', 0), 'star': row.get('star') or 1} for key, row in heroes.items()},
        'pvp': {key: dict(pvp[str(row['act_param'][0])]) for key, row in activities.items() if row.get('act_type') == 2 and sequence(row.get('act_param')) and str(row['act_param'][0]) in pvp},
        'item_types': {key: row.get('type', 0) for key, row in items.items()},
        'bonds': {key: {'heroes': row.get('heroes') or [], 'levels': [{'hero_num': level['hero_num'], 'extra_attr': level.get('extra_attr') or {}} for level in bond_levels if level['bonds_type'] == int(key)]} for key, row in load('bond_type_data_cn').items()},
        'exchange_heroes': sorted(int(key) for key, row in heroes.items() if row.get('star') == 3 and 'hero' in sequence(row.get('show_type')) and 'classifyMark' in row),
        'manual': manual(t),
    }


def flatten(value):
    if isinstance(value, list):
        for child in value:
            yield from flatten(child)
    elif isinstance(value, dict):
        for child in value.values():
            yield from flatten(child)
    elif isinstance(value, int) and not isinstance(value, bool):
        yield value


def manual(t):
    load = t.module
    characters, books, waves, towers = load('character_data_new_cn'), load('book_monster_data_cn'), load('battle_wave_data'), load('td_wave_data')
    monsters = {key: row['relatedId'] for key, row in characters.items() if isinstance(row.get('relatedId'), int)}
    monsters.update({key: int(key) for key in books if key not in monsters})
    battles = {}
    for key, row in load('battle_mode_new_data_cn').items():
        enemies = set(flatten(row.get('enemy')))
        for wave in flatten(row.get('wave')):
            enemies.update(flatten(waves.get(str(wave), {}).get('enemy')))
        for wave in flatten(row.get('td_wave')):
            enemies.update(entry[0] for entry in sequence(towers.get(str(wave), {}).get('enemy')) if isinstance(entry, list) and entry)
        found = sorted({monsters[str(enemy)] for enemy in enemies if str(enemy) in monsters})
        if found:
            battles[key] = found
    rule = sequence(load('constant_data').get('dispatch_egg_rule'))
    return {
        'monsters': monsters,
        'battles': battles,
        'artifact_suits': {key: row['suit'] for key, row in load('artifact_data_cn').items() if isinstance(row.get('suit'), int)},
        'eggs': sorted(int(key) for key, row in load('dispatch_egg_hand_book_data_cn').items() if row.get('handbook')),
        'egg_chance': rule[0] if rule else 0,
        'egg_new_chance': rule[1] if len(rule) > 1 else 0,
    }


def tasks(t):
    load = t.module
    daily = {row['taskId'] for row in load('daily_task_data_cn').values()}
    guild = {row['taskid'] for row in load('alliance_task_data_cn').values()}
    limited = {row['task'] for row in load('limited_time_event_battlepass_task_data_cn').values()}
    hyper = {row['taskid'] for row in load('hyperflow_daymission_data_cn').values()}
    battlepass = {row['task'] for row in load('battlepass_task_data').values()}
    achievements = {row['taskId']: row for row in load('achievement_data_cn').values()}
    bosses = load('limited_time_event_boss')
    favors = load('rogue_pve_buff_data_cn')
    rows = load('task_data')
    ids = {int(key) for key in rows}
    referenced = set()

    def walk(value):
        if isinstance(value, bool):
            return
        if isinstance(value, int):
            if value in ids:
                referenced.add(value)
        elif isinstance(value, str):
            if value.isdigit() and int(value) in ids:
                referenced.add(int(value))
        elif isinstance(value, dict):
            for child in value.values():
                walk(child)
        elif isinstance(value, list):
            for child in value:
                walk(child)

    for _, data in t.data_tables():
        walk(data)
    referenced.update(value for value in t.code_constants() if value in ids)
    result = {}
    for key, row in rows.items():
        parameters = sequence(row.get('param'))
        number = int(key)
        kind = 'guild' if number in guild else 'hyperflow' if number in hyper else 'limited_pass' if number in limited else 'battle_pass' if number in battlepass else ''
        task = {**row, 'param': ints(parameters), 'param_lists': {str(index): value for index, value in enumerate(parameters) if isinstance(value, list) and all(isinstance(item, int) for item in value)}, 'param_texts': {str(index): value for index, value in enumerate(parameters) if isinstance(value, str)}, 'daily': number in daily, 'scope': kind}
        if number not in referenced:
            task['unreferenced'] = True
        if number in achievements:
            task['achievement'], task['points'] = achievements[number]['id'], achievements[number].get('points', 0)
        if row['complete_type'] == 88 and parameters:
            task['targets'] = sorted(int(id) for id, favor in favors.items() if favor.get('type') == parameters[0])
        if row['complete_type'] == 112 and parameters and isinstance(parameters[0], list):
            task['targets'] = sorted(battle for boss in parameters[0] for battle in sequence(bosses.get(str(boss), {}).get('battle')))
        if row['complete_type'] == 126 and 'reward' not in row and len(parameters) > 2:
            task['reward'] = parameters[2]
        result[key] = task
    return result


def journey(t):
    load = t.module
    constants = load('constant_data')

    def passes(data_name, reward_name):
        rewards = load(reward_name)
        result = {}
        for key, row in load(data_name).items():
            price = sequence(row.get('battlepass_lv_buy'))
            result[key] = {'start': row.get('start_time', 0), 'end': row.get('end_time', 0), 'maximum_level': row['battlepass_lv_cap'], 'experience_per_level': row['battlepass_lv_expup'], 'weekly_cap': row.get('weekly_experience_cap', 0), 'currency': price[0] if price else 0, 'level_price': price[1] if len(price) > 1 else 0, 'tasks': sequence(row.get('current_task')), 'weekly_tasks': sequence(row.get('week_task')), 'rewards': {str(value['level']): {'free': value.get('reward1', 0), 'paid': sequence(value.get('reward2'))} for value in rewards.values() if value['affiliated_pass'] == int(key)}}
        return result

    brawls = load('activity_brawl_data_cn')
    hyper_tasks = load('hyperflow_daymission_data_cn')
    conditions = {}
    for row in sorted(load('act_condition_task_data').values(), key=lambda row: row['id']):
        conditions.setdefault(str(row['activity_id']), []).append({'tasks': sequence(row.get('taskIds')), 'daily': bool(row.get('daily')), 'once': bool(row.get('once_task'))})
    activities = {}
    for key, row in load('activity_data').items():
        parameters = sequence(row.get('act_param'))
        value = {'type': row['act_type'], 'level': row.get('open_lv', 0), 'start': row.get('start_time', 0), 'end': row.get('end_time', 0), 'duration': row.get('duration', 0), 'parameter': parameters[0] if parameters and isinstance(parameters[0], int) else 0}
        if row.get('loginday') and row.get('loginlv'):
            value['login_day'], value['login_level'] = row['loginday'], row['loginlv']
        if follows := [other for other in ints(parameters[1:]) if str(other) != key and load('activity_data').get(str(other), {}).get('act_type') == row['act_type']]:
            value['follows'] = follows[0]
        if sequence(row.get('act_task')):
            value['tasks'] = sequence(row.get('act_task'))
        if key in conditions:
            value['conditions'] = conditions[key]
        if row['act_type'] == 12 and len(parameters) > 1:
            value['rotation'] = sequence(parameters[1])
        if row['act_type'] == 32 and len(parameters) > 1:
            value['permanent'] = sequence(parameters[1])
        if row['act_type'] == 25 and len(parameters) > 3:
            value['coin'], value['weekly_cap'] = parameters[2], parameters[3]
        if row['act_type'] == 24 and len(parameters) > 1:
            value['rewards'] = {str(index + 1): reward for index, reward in enumerate(parameters[1])}
        if row['act_type'] == 10 and parameters:
            brawl = brawls[str(parameters[0])]
            value['thresholds'] = {str(index): star for index, star in enumerate(brawl['starNum'])}
            value['rewards'] = {str(index): reward for index, reward in enumerate(brawl['reward'])}
            value['battles'] = brawl['battleId']
            value['environments'] = sequence(brawl.get('environment'))
        activities[key] = value
    return {
        'challenge_early_level_margin': constants['challenge_less_lv'],
        'minigame_unlock_mission': min(row['unlock_mission'] for row in load('limited_time_event').values() if row.get('event_type') == 4),
        'limited_event_passes': sorted(value for row in load('limited_time_event').values() if row.get('event_type') == 2 for value in ints(row.get('event_param'))),
        'novice_tasks': {key: {'nodes': sequence(row.get('initNodes')), 'next': sequence(row.get('nextId'))} for key, row in load('mainplot_task_data').items()},
        'novice_nodes': {key: {'type': row['eventType'], 'parameters': sequence(row.get('params')) if all(isinstance(value, int) for value in sequence(row.get('params'))) else []} for key, row in load('mainplot_node_data_cn').items()},
        'passes': passes('battlepass_data', 'battlepass_reward_data'),
        'limited_passes': passes('limited_time_event_battlepass_data', 'limited_time_event_battlepass_reward_data'),
        'limited_bosses': {key: {'battles': row['battle'], 'cost': row['cost'], 'open': row['open_time'], 'end': row['end_time'], 'rewards': row['fixed_reward'], 'level_reward': row['level_reward']} for key, row in load('limited_time_event_boss').items()},
        'limited_boss_rewards': {key: {'tag': row['tag'], 'levels': row['level'], 'reward': row['reward']} for key, row in load('limited_time_event_boss_reward').items()},
        'activities': activities,
        'trailers': {key: {'level': row.get('limit_lv', 0), 'days': row.get('limit_login', 0), 'reward': row['reward']} for key, row in load('game_trailer_data_cn').items()},
        'hyperflows': {key: {'buff_nodes': row['buff_firstnode'], 'battle_nodes': row['battle_firstnode'], 'tasks': [hyper_tasks[str(task)]['taskid'] for task in row['daymission']]} for key, row in load('hyperflow_data').items()},
        'hyper_buffs': {key: {'type': row['type'], 'choices': sequence(row.get('includenode')), 'threshold': row.get('unlockpoint', 0)} for key, row in load('hyperflow_buff_node_data').items()},
        'hyper_battles': {key: {'battles': row['battlemode_id'], 'next': row.get('nextnode', 0), 'rewards': [sequence(row.get('star1Reward')), sequence(row.get('star2Reward')), sequence(row.get('star3Reward'))]} for key, row in load('hyperflow_battleNode_data_cn').items()},
        'invite_rewards': {key: row['reward'] for key, row in load('referral_growth_task_data_cn').items()},
        'invite_count_tasks': {key: row['taskId'] for key, row in load('referral_invitation_task_data_cn').items()},
        'invite_bind_reward': constants['bind_pull_system_reward'],
        'invite_tasks': {str(row['index']): row['taskId'] for row in load('referral_newplayer_task_data_cn').values() if 'index' in row},
        'invite_share_task': min(row['taskId'] for row in load('referral_newplayer_task_data_cn').values() if 'index' not in row),
        'monthly_purchase_reward': constants['monthly_pass_purchase_reward'],
        'monthly_daily_reward': constants['monthly_pass_daily_reward'],
        'monthly_duration': constants['monthly_pass_duration_obtained'],
        'monthly_maximum_duration': constants['monthly_pass_max_duration'],
        **dreamguard(t),
        'challenge_types': {key: sequence(row.get('group')) for key, row in load('challenge_type_data_cn').items()},
        'challenges': {key: {'difficulty': row['lv'], 'group': row['group'], 'battle': row['battle_mode'], 'level': row.get('unlock_level', 0), 'predecessor': row.get('befId', 0), 'energy': row.get('energy', 0), 'active': row.get('active', 0), 'rewards': sequence(row.get('reward_once')), 'star_rewards': [row.get('reward_1', 0), row.get('reward_2', 0), row.get('reward_3', 0)]} for key, row in load('challenge_data').items()},
        'spiral_groups': {key: {'missions': row['mission_group'], 'rewards': row.get('star_reward') or {}} for key, row in load('spiral_group_data').items()},
        'spiral_missions': {key: {'battle': row['battle_id'], 'predecessor': row.get('bf_mission', 0), 'rewards': [row.get('reward1', 0), row.get('reward2', 0), row.get('reward3', 0)]} for key, row in load('spiral_mission_data_cn').items()},
        'solo_groups': {key: {'nodes': row['battle_node'], 'rewards': row['star_reward']} for key, row in load('single_battle_data').items()},
        'solo_nodes': {key: row['battlemode_id'] for key, row in load('single_battle_node_data_cn').items()},
        'minigames': {key: {'predecessor': row.get('pre_level', 0), 'start': row.get('unlock_time', 0), 'level': row.get('unlock_lv', 0), 'reward': row['first_reward']} for key, row in load('limited_time_event_minigame_data_cn').items()},
        'missions': {key: {'level': row.get('unlockLv', 0), 'predecessors': sequence(row.get('befMission')), 'first_node': row['firstInitNode'], 'reward': row.get('firstReward', 0)} for key, row in load('limited_time_event_mission_data').items()},
        'mission_nodes': {key: {'next': row.get('nextNode', 0), 'type': row['nodeType'], 'parameter': row.get('param', 0) if isinstance(row.get('param'), int) else 0} for key, row in load('limited_time_event_node_data').items()},
        'seven_day': seven_day(t),
    }


def dreamguard(t):
    task, duration, _, reward = sequence(t.module('constant_data')['rallying_cry_hero_task_params'])
    pays = [sequence(row.get('pay')) for row in t.module('rallying_cry_data_cn').values() if str(task) in [str(value) for value in sequence(row.get('access_param'))]]
    return {'dreamguard_task': task, 'dreamguard_duration': duration, 'dreamguard_price': {str(pay[0]): pay[1] for pay in pays if len(pay) > 1}, 'dreamguard_reward': reward}


def seven_day(t):
    rows, tasks = list(t.module('newbie_task_data_cn').values()), t.module('task_data')
    marks = {ints(tasks[str(row['taskId'])].get('param'))[0] for row in rows if 'day' not in row}
    return {'mark': marks.pop() if len(marks) == 1 else 0, 'tasks': sorted(row['taskId'] for row in rows if 'day' in row)}


def code_field(t, module, field):
    def walk(node):
        yield node
        for child in node['children']:
            yield from walk(child)

    root = parse((t.folder / t.index[module + '.lua']['file']).read_bytes())[1]
    for node in walk(root):
        constants, loads = node['constants'], {}
        for instruction in node['code']:
            op, register = t.tools.opcodes[instruction & 63], instruction >> 6 & 255
            if op == 'LOADK':
                loads[register] = instruction >> 14
                continue
            if op != 'SETTABLE':
                loads.pop(register, None)
                continue
            key, value = instruction >> 23 & 511, instruction >> 14 & 511
            key = key - 256 if key >= 256 else loads.get(key)
            value = value - 256 if value >= 256 else loads.get(value)
            if key is None or value is None:
                continue
            if constants[key][0] in (4, 20) and constants[key][1] == field.encode() and constants[value][0] == 19:
                return struct.unpack('<q', constants[value][1])[0]
    raise ValueError(module + '.' + field)


def rogue_chest_drops(t):
    load = t.module
    items, rewards, strings = load('item_data_cn'), load('reward_data'), load('error_content_cn')
    chests = load('rogue_chest_data_cn')
    boxes = {str(row[0]) for chest in chests.values() for reward in sequence(chest.get('reward')) for row in sequence(rewards.get(str(reward), {}).get('preview')) if isinstance(row, list) and len(row) >= 2 and row[1] == 0}
    names = {item: items[item]['name'] for item in boxes if items.get(item, {}).get('type') == 1029}
    prefix = os.path.commonprefix(list(names.values()))
    classes = {item: name[len(prefix):] for item, name in names.items()}
    heading = re.escape(strings['PANEL_ROGUE_CHEST_LEVEL']['error_content']).replace(re.escape('{1}'), r'(\d+)')
    rates = {}
    for line in re.sub(r'<[^>]+>', '', strings['PANEL_ROGUE_CHEST_HELP_CN']['error_content']).split('\n'):
        level = re.search(heading, line)
        if not level:
            continue
        guaranteed, extra = {}, {}
        for share, segment in re.findall(r'(\d+(?:\.\d+)?)%(.*?)(?=\d+(?:\.\d+)?%|$)', line[level.end():]):
            found = [word for word in set(classes.values()) if word in segment]
            if len(found) != 1:
                continue
            count = re.search(r'(\d+)\D?' + re.escape(found[0]), segment)
            if float(share) == 100:
                guaranteed[found[0]] = int(count.group(1)) if count else 1
            else:
                extra[found[0]] = round(float(share) * 10000)
        rates[level.group(1)] = {'guaranteed': guaranteed, 'extra': extra}
    result, previous = {}, None
    for key in sorted(chests, key=int):
        current = rates.get(str(chests[key].get('levelDes')), previous)
        previous = current
        if current is None:
            continue
        for reward in sequence(chests[key].get('reward')):
            rows = {classes[str(row[0])]: str(row[0]) for row in sequence(rewards.get(str(reward), {}).get('preview')) if isinstance(row, list) and len(row) >= 2 and row[1] == 0 and str(row[0]) in classes}
            result[str(reward)] = {
                'guaranteed': {rows[word]: count for word, count in current['guaranteed'].items() if word in rows},
                'extra': {rows[word]: chance for word, chance in current['extra'].items() if word in rows},
            }
    return result


def rogue(t):
    load = t.module
    constants = load('constant_data')

    def rows(name, fields):
        return {key: {field: value[field] for field in fields if field in value} for key, value in load(name).items()}

    events = rows('rogue_pve_event_data_cn', ['action1', 'action2', 'action3', 'param1', 'param2', 'param3'])
    for row in events.values():
        for key in ['param1', 'param2', 'param3']:
            row[key] = sequence(row.get(key))
    shops = rows('rogue_shop_data', ['relic_num', 'relic_coin', 'add_hero', 't1_lv1_prob', 't2_lv1_prob', 't3_lv1_prob', 't4_lv1_prob'])
    for row in shops.values():
        row['weights'] = [row.pop('t%d_lv1_prob' % tier, 0) for tier in range(1, 5)]
    relics = rows('rogue_relic_data_cn', ['tier', 'type', 'heroLvUp', 'skill', 'shopSkill'])
    for row in relics.values():
        for key in ['skill', 'shopSkill']:
            row[key] = [sequence(value) for value in row.get(key, [])]
    battle = load('battle_constant_data')
    unlock = load('system_unlock_data')
    return {
        'alternations': {key: {'type': row['type'], 'start': row['start_time'], 'coin_limit': row.get('coins_limit', 0), 'levels': ints(row.get('include_levelid')), 'rewards': row.get('reward') or {}} for key, row in load('rogue_alternation_data').items()},
        'unlock': {key: {'level': unlock[key].get('lv', 0), 'login_day': unlock[key].get('loginday', 0), 'login_level': unlock[key].get('loginlv', 0)} for key in ['roguelike', 'roguelikePve']},
        'score': constants['rogue_score'],
        'pve_score': constants['rogue_pve_score'],
        'record_limit': constants['rogue_record_len'],
        'hero_star': battle['rogue_hero_star'],
        'max_hero_level': battle['rogue_max_hero_lv'],
        'max_time_score': code_field(t, 'Const', 'REDIS_MAX_FLOAT'),
        'currency': code_field(t, 'Const', 'ROGUE_CURRENCY'),
        'pve_currency': code_field(t, 'Const', 'ROGUE_PVE_CURRENCY'),
        'runs': rows('rogue_data', ['battle_seq', 'battle_pool', 'hero_pool', 'relic_pool', 'shop_lv_seq', 'chest', 'battle_lv_seq']),
        'shops': shops,
        'relics': relics,
        'chests': rows('rogue_chest_data_cn', ['energy_cost', 'coin_cost', 'limitLv', 'unlock_battle', 'reward']),
        'chest_drops': rogue_chest_drops(t),
        'hero_unlock_wins': {key: row.get('unlock_win_num', 0) for key, row in load('rogue_unlock_hero_data').items()},
        'hero_num': constants['rogue_hero_num'],
        'pve': rows('rogue_pve_data', ['battle_seq', 'battle_lv_seq', 'evt_group_seq', 'evt_group_pool', 'buff_pool', 'enemy_buff_seq', 'unlock_lv', 'difficulty', 'opening_time', 'posList', 'posSwitch']),
        'events': events,
        'event_groups': {key: value['event'] for key, value in load('rogue_pve_event_group_data').items()},
        'buffs': rows('rogue_pve_buff_data_cn', ['tier', 'cost', 'buffid']),
        'first_rewards': constants['rogue_first_reward'],
        'week_rewards': constants['rogue_week_reward'],
        'pve_first_rewards': constants['rogue_pve_first_reward'],
        'pve_week_rewards': constants['rogue_pve_week_reward'],
        'win_reward': constants['rogue_win_reward'],
        'pve_win_rewards': constants['rogue_pve_win_reward'],
        'relic_prices': constants['rogue_relic_gold_cost'],
        'refresh_price': constants['rogue_refresh_gold_cost'],
        'sale_price': constants['rogue_sale_gain'],
        'life': constants['rogue_life'],
        'hero_level': constants['rogue_hero_level'],
        'rechoose_limit': constants['rogue_hero_replace_num'],
        'max_wins': constants['rogue_win_num'],
    }


def draw(t):
    load = t.module
    sets = load('draw_prizeset_data')
    levels = sorted({int(field[7:]) for row in sets.values() for field in row if field.startswith('weight_') and field[7:].isdigit()})
    return {
        'pools': {key: {'ticket': row['ticket'], 'price': row.get('pricePerDraw', 0), 'level': row.get('unlockLv', 1), 'start': row.get('beginTime', 0), 'end': row.get('endTime', 0), 'sets': row.get('prizeSet') or [], 'strategies': row.get('strategy') or [], 'weight_strategy': row.get('weight_fix_strategy', 0), 'point': row.get('pointType', 0)} for key, row in load('draw_pool_data').items()},
        'sets': {key: {'items': row.get('items') or [], 'up': row.get('ups') or [], 'weight': row.get('weight', 0), 'soft_weights': {str(level): row.get('weight_' + str(level), row.get('weight', 0)) for level in levels}} for key, row in sets.items()},
        'strategies': {key: {'counter': row.get('counter', 0), 'number': row.get('num', 0), 'sets': row.get('set') or [], 'prizes': row.get('prize') or [], 'trigger_sets': row.get('trigger_set') or [], 'trigger_prizes': row.get('trigger_prize') or [], 'advance_prizes': row.get('advance_prize') or [], 'advance_number': row.get('advance_num', 0), 'advance_trigger': row.get('advance_trigger_prize') or [], 'created': row.get('create_time', 0)} for key, row in load('draw_strategy_data').items()},
    }


def events(t):
    load = t.module
    return {
        'science': {key: {'next': row.get('nextNode') or [], 'choices': row.get('includeNode') or []} for key, row in load('season_science_data_cn').items()},
        'season_levels': {key: {str(level): row.get('exp', 0) for level, row in value.items()} for key, value in load('season_level_data').items()},
        'boss_opens': {key: {'start': row.get('begin_time', 0), 'end': row.get('end_time', 0), 'settle': row.get('settle_time', 0), 'event': row['event']} for key, row in load('boss_open_data').items()},
        'bosses': {key: {'monster': row['monster'], 'missions': row.get('mission') or []} for key, row in load('boss_event_data_cn').items()},
        'boss_missions': {key: {'battle': row['battle'], 'base': row.get('base_point', 0), 'time': row.get('time_point', 0), 'limit': row.get('time_limit', 0), 'reward': row.get('first_reward', 0)} for key, row in load('boss_mission_data').items()},
        'boss_scores': {key: row['reward'] for key, row in load('boss_score_data').items()},
        'boss_ranks': {key: {'minimum': row['param_min'], 'maximum': row['param_max'], 'min_percent': row.get('param_type_min') == 'percent', 'max_percent': row.get('param_type_max') == 'percent', 'reward': row['reward']} for key, row in load('boss_rank_data').items()},
    }


RALLYING_CRY = 18


def gift_triggers(tasks, ids):
    result = []
    for task in ints(ids):
        row = tasks.get(str(task), {})
        if row.get('complete_type') == 3:
            result.append({'level': row.get('target', 0)})
        elif row.get('complete_type') == 65 and ints(row.get('param')):
            result.append({'gift': ints(row.get('param'))[0]})
    return result


def gifts(t):
    load = t.module
    tasks = load('task_data')
    rallies = sorted(int(key) for key, row in load('activity_data').items() if row.get('act_type') == RALLYING_CRY)
    offered = {int(value) for row in load('rallying_cry_data_cn').values() if row.get('access') == 'gift' for value in sequence(row.get('access_param')) if str(value).isdigit()}
    result = {}
    for key, row in load('gift_data').items():
        entry = {'product': row.get('recharge_id', 0), 'reward': row.get('giftReward', 0), 'duration': row.get('giftLifeTime', 0) * 60}
        triggers = gift_triggers(tasks, row.get('giftTriggerTask'))
        if int(key) in offered:
            triggers += [{'activity': activity} for activity in rallies]
        if triggers:
            entry['triggers'] = triggers
        if retriggers := gift_triggers(tasks, row.get('ReTriggerTask')):
            entry['retriggers'] = retriggers
        result[key] = entry
    return result


def system(t):
    load = t.module
    recharges = load('recharge_data')
    products = {}
    for row in sorted(recharges.values(), key=lambda value: value.get('area', 0)):
        products[str(row['product_id'])] = {'yuanbao': row.get('yuanbao', 0), 'bonus': row.get('yuanbao_free', 0), 'monthly': row.get('des') == 'monthlypass'}
    fields = ['recharge_id', 'discount_recharge_id', 'discount_start', 'discount_end', 'unlock_lv', 'limit_type', 'limit_param', 'period_time', 'start_time', 'end_time', 'reward', 'before_shop', 'shop_type']
    return {
        'products': products,
        'shops': {key: {field: row[field] for field in fields if field in row} for key, row in load('shop_data_cn').items() if row.get('recharge_id')},
        'passes': {key: {str(row['recharge_id' + str(index)]): {'kind': index, 'reward': row.get('recharge_reward' + str(index), 0)} for index in range(1, 4) if 'recharge_id' + str(index) in row} for key, row in load('battlepass_data').items()},
        'gifts': gifts(t),
        'hero_packs': {key: {'product': row.get('recharge_id', 0), 'discount': row.get('discount_recharge_id', 0), 'item': row.get('itemid', 0)} for key, row in load('hero_purchase_pack_data').items()},
        'recharges': {key: row['product_id'] for key, row in recharges.items()},
    }


def gm(t, current):
    load = t.module
    mail = {}
    previous = current.get('mail', {})
    for key, row in load('mail_data_cn').items():
        if row.get('link'):
            continue
        entry = {'title': row.get('title', 'GM mail'), 'content': row.get('content', 'GM mail'), 'sender': row.get('sender', 'Local GM'), 'reward': row.get('rewardId', 0)}
        for field in ('title', 'content', 'sender'):
            if field in previous.get(key, {}):
                entry[field] = previous[key][field]
            elif isinstance(entry[field], str) and entry[field].startswith('!'):
                entry[field] = entry[field][1:]
        mail[key] = entry
    return {
        'guides': sorted({str(row.get('SaveCond') or key) for key, row in load('guide_data').items() if row.get('BootOn')}),
        'chapters': {key: row.get('tag', '') for key, row in load('dreamland_chapter_data').items()},
        'mail': mail,
    }


def catalog(t):
    load = t.module
    books = load('book_character_data_cn')
    heroes = {key: {'star': row.get('star') or 1, 'talents': {str(index): 1 for index in range(1, 6)}} for key, row in load('character_data_new_cn').items() if 'hero' in sequence(row.get('tag')) and key in books}
    unreleased = {str(row.get('itemId')) for name in ('artifact_data_cn', 'highlight_data_cn') for key, row in load(name).items() if key not in t.released(name)}
    return {
        'activities': sorted(int(key) for key in load('activity_data')),
        'artifacts': sorted(int(key) for key in t.released('artifact_data_cn')),
        'heroes': heroes,
        'highlights': sorted(int(key) for key in t.released('highlight_data_cn')),
        'items': sorted(int(key) for key in load('item_data_cn') if key not in unreleased),
        'skins': sorted(int(key) for key in load('skin_res_data_cn')),
    }


def network(t):
    load = t.module
    mains, bosses = load('pvp_main_data'), load('alliance_war_boss_data_cn')
    wars = {}
    for key, row in load('alliance_war_data_cn').items():
        boss = bosses[str(row['boss'])]
        areas = [row['area' + str(index)] for index in range(1, 5) if 'area' + str(index) in row]
        wars[key] = {'areas': areas, 'boss_group': boss['battle_group'], 'boss_hp': boss['hp'], 'boss_reward': row.get('boss_finish_reward', 0), 'area_rewards': {str(row['area' + str(index)]): row.get('area%d_finish_reward' % index, 0) for index in range(1, 5) if 'area' + str(index) in row}}
    activities, rankings = {}, {}
    for key, row in load('activity_data').items():
        parameters = sequence(row.get('act_param'))
        if row['act_type'] != 14 or not parameters or str(parameters[0]) not in mains:
            continue
        main = mains[str(parameters[0])]
        rankings[key] = {'reward': main.get('ranking_reward', 0), 'limit': main.get('request_ranking', 0)}
        activities[key] = {'battles': sequence(main.get('battle')), 'attempts': main.get('day_recover', 0), 'refreshes': main.get('day_refresh', 0), 'tasks': sequence(main.get('point_task')), 'environments': sequence(main.get('environment')), 'camps': sequence(main.get('camp_environment'))}
    return {
        'ranks': {key: {'minimum': row['point_min'], 'maximum': row['point_max'], 'reward': row.get('rank_reward', 0), 'win': row.get('battle_win_point', 0), 'lose': row.get('battle_lose_point', 0), 'kill': row.get('atk_hero_kill_point', 0), 'killed': row.get('atk_hero_killed_point', 0), 'all_kill': row.get('atk_allkill_point', 0), 'all_killed': row.get('atk_allkilled_point', 0), 'ai_level': row.get('ai_level', 0)} for key, row in load('pvp_data_cn').items()},
        'activities': activities,
        'wars': wars,
        'areas': {key: {'hp': row['hp'], 'sets': [row['set' + str(index)] for index in range(1, 4) if 'set' + str(index) in row]} for key, row in load('alliance_war_area_data_cn').items()},
        'war_levels': {key: {'group': row['group'], 'battle': row['battle_id'], 'minimum_level': row['lv_limit'][0], 'maximum_level': row['lv_limit'][1], 'reduction': row.get('hp_reduce', 0), 'medals': row.get('medal', 0), 'assist': row.get('assist', 0), 'reward': row.get('battle_reward', 0)} for key, row in load('alliance_war_battle_group_data').items()},
        'bots': {key: {'heroes': {str(int(template) // 100): level for template, level in row['lineup'].items()}, 'environment': row.get('ai_env', 0), 'camp': row.get('ai_camp_env', 0)} for key, row in load('pvp_ai_lineup_data').items()},
        'rank_seasons': {key: {'reset': row['reset_point'], 'environments': row['env_num']} for key, row in load('pvp_data_cn').items()},
        'initial_score': load('constant_data')['PVP_init_point'],
        'season_rankings': rankings,
    }


def activity_rows(t, kind):
    result = {}
    for key, row in t.module('activity_data').items():
        parameters = sequence(row.get('act_param'))
        if row['act_type'] == kind:
            result[key] = {'type': kind, 'level': row.get('open_lv', 0), 'start': row.get('start_time', 0), 'end': row.get('end_time', 0), 'parameter': parameters[0] if parameters and isinstance(parameters[0], int) else 0}
    return result


def retail_pvp(t):
    load = t.module
    constants = load('constant_data')
    mine_fields = ['base_point', 'base_mine', 'lower_limit_eff', 'auto_grow', 'upper_limit_eff', 'init_eff', 'bonus_eff', 'minus_eff', 'show_hero', 'battle_mode']
    return {
        'activities': activity_rows(t, 31),
        'main': load('multi_pvp_main_data'),
        'points': load('multi_pvp_point_data'),
        'mines': {key: {field: row[field] for field in mine_fields if field in row} for key, row in load('multi_pvp_mine_data_cn').items()},
        'ai': load('multi_pvp_ai_lineup_data'),
        'daily_hero_attacks': constants['multi_pvp_daily_hero_attack'],
        'maximum_revenge': constants['multi_pvp_max_revenge_times'],
        'daily_defenses': constants['multi_pvp_daily_defend'],
        'settle_days': {key: row['act_param'][1] for key, row in load('activity_data').items() if row['act_type'] == 31 and len(sequence(row.get('act_param'))) > 1},
    }


def retail_activity(t):
    load = t.module
    constants, rewards, items = load('constant_data'), load('reward_data'), load('item_data_cn')
    nodes = load('camp_challenge_node_data')
    scores = load('camp_challenge_score_data')
    granted = sorted({reward for row in nodes.values() for reward in sequence(row.get('reward_id'))})
    score_items = sorted({row['reward_item'] for row in scores.values()} | {int(item) for row in nodes.values() for item in (row.get('cost') or {})})
    return {
        'activities': activity_rows(t, 35),
        'camp_groups': {key: sequence(row.get('battlenode')) for key, row in load('camp_challenge_data').items()},
        'camp_nodes': {key: {'battles': sequence(row.get('battlemode_id')), 'difficulties': sequence(row.get('difficulty')), 'camp': row.get('camp_limit', 0), 'cost': row.get('cost') or {}, 'days': sequence(row.get('openning_time')), 'rewards': sequence(row.get('reward_id'))} for key, row in nodes.items()},
        'camp_scores': {key: {'base': row['reward_base_num'], 'per_second': row['reward_time_num'], 'limit': row['time_limit'], 'item': row['reward_item']} for key, row in scores.items()},
        'rewards': {str(reward): [value[:2] for value in sequence(rewards.get(str(reward), {}).get('preview')) if isinstance(value, list) and len(value) >= 2] for reward in granted},
        'items': {str(item): {'type': items[str(item)]['type'], 'rarity': items[str(item)].get('rarity', 0)} for item in score_items},
        'daily_refresh': constants['daily_logic_refresh_sec'],
        'invite_maximum': constants['invite_system_params'][0],
        'invite_reward_maximum': constants['invite_system_params'][1],
    }


def retail(t):
    load = t.module
    constants = load('constant_data')
    unlock = load('system_unlock_data')
    items = {}
    for item in sequence(constants['init_artifact']) + sequence(constants['init_highlight']):
        items[str(item)] = items.get(str(item), 0) + 1
    points = load('daily_task_progress_data')
    point_items = {ints(load('task_data')[str(row['taskId'])].get('param'))[0] for row in points.values()}
    return {
        'level': min(int(key) for key in load('player_level_data')),
        'energy': constants['init_energy'],
        'items': items,
        'head': sequence(constants['init_head'])[0],
        'frame': sequence(constants['init_head_frame'])[0],
        'background': sequence(constants['init_background'])[0],
        'energy_seconds': constants['energy_recover_time'],
        'bank_limit': constants['energy_bank_limit'],
        'bank_seconds': constants['energy_bank_time'],
        'team_breaks': {key: {'battle': row['battle'], 'reward': row.get('reward', 0), 'task': unlock.get('teamBreak_' + key, {}).get('task_id', 0)} for key, row in sorted(load('team_break_data').items(), key=lambda pair: int(pair[0]))},
        'default_skins': sorted(int(key) for key, row in load('skin_res_data_cn').items() if row.get('isDefault')),
        'daily': {
            'newbie_level': constants['newbie_daily_task_lv'],
            'newbie_tasks': sequence(constants['newbie_daily_tasks']),
            'counts': {str(key): value for key, value in (mapping(constants['daily_task_type_num']) or {}).items()},
            'tasks': [{'task': row['taskId'], 'type': row['dailyType'], 'level': row.get('teamLv', 1), 'weight': row.get('weight', 0)} for _, row in sorted(load('daily_task_data_cn').items(), key=lambda pair: int(pair[0]))],
            'point_tasks': sorted(row['taskId'] for row in points.values()),
            'point_item': point_items.pop() if len(point_items) == 1 else 0,
        },
    }


def constants(t):
    load = t.module
    return {
        'constants': load('constant_data'),
        'system_unlocks': {key: {'level': row.get('lv', 0), 'login_day': row.get('loginday', 0), 'login_level': row.get('loginlv', 0), 'mission': row.get('missionId', 0), 'task': row.get('task_id', 0)} for key, row in load('system_unlock_data').items() if isinstance(row, dict)},
        'hero_star_rewards': {key: row['star3_reward'] for key, row in load('hero_star_reward_data').items() if row.get('star3_reward')},
        'guild_levels': {key: {'experience': row.get('exp', 0), 'members': row['members'], 'reward': row['reward'], 'minimum': row.get('reward_min', 0), 'per_reward': row.get('contri_to_reward', 0), 'reward_limit': row.get('reward_limit', 0)} for key, row in load('alliance_level_data').items()},
        'item_ids': t.code_module('Const')['ItemId'],
        'honor_tags': sorted(t.code_module('UIConst')['ServerHonorType'], key=t.code_module('UIConst')['ServerHonorType'].get),
        'formation_size': t.code_module('Const')['FormationHeroMaxCount'],
        'guild_wars': {key: {'prepare': row['prepare_time'], 'battle': row['battle_time'], 'end': row['end_time'], 'phases': sequence(row.get('phase_change'))} for key, row in load('alliance_war_data_cn').items() if row.get('prepare_time') and row.get('battle_time') and row.get('end_time')},
    }


KEEP = {
    ('rules.json', 'battle_settings'): ('skip_deploy', 'template_heroes'),
}


def merge_rows(old, new, keep=()):
    if not isinstance(old, dict) or not isinstance(new, dict):
        return new
    result = {}
    for key in [key for key in old if key in new] + [key for key in new if key not in old]:
        row = new[key]
        if isinstance(row, dict) and isinstance(old.get(key), dict):
            row = {**row, **{field: old[key][field] for field in keep if field in old[key]}}
            row = {**{field: row[field] for field in old[key] if field in row}, **row}
        result[key] = row
    return result


def merge_section(name, key, old, new):
    if (name, key) == ('rules.json', 'memento_maps'):
        return {map_id: merge_rows(old.get(map_id, {}), tiles, ('team',)) for map_id, tiles in merge_rows(old, new).items()}
    if (name, key) == ('gameplay_defaults.json', 'retail'):
        result = dict(old)
        for field, value in new.items():
            result[field] = {**old.get(field, {}), **value} if field == 'daily' else value
        return result
    return merge_rows(old, new, KEEP.get((name, key), ()))


def changes(old, new):
    if not isinstance(old, dict) or not isinstance(new, dict):
        return {'changed': int(old != new)}
    return {'added': sum(key not in old for key in new), 'removed': sum(key not in new for key in old), 'changed': sum(key in new and old[key] != new[key] for key in old)}


def write(path, data, raw):
    if raw.startswith('{\n') or raw.startswith('{\r\n'):
        text = json.dumps(data, ensure_ascii=False, indent=2)
    else:
        text = json.dumps(data, ensure_ascii=False, separators=(',', ':'))
    text += raw[len(raw.rstrip('\r\n')):].replace('\r\n', '\n') if raw else '\n'
    path.write_text(text, encoding='utf-8', newline='')
    return text


def build(arguments):
    folder = arguments.repo / DATATABLES
    tools = Toolchain(arguments.toolchain)
    legacy = arguments.repo / 'assets' / LEGACY
    t = Tables(tools, arguments.analysis, json.loads(legacy.read_text(encoding='utf8'))['fields'] if legacy.is_file() else {})
    exporters = {
        'catalog.json': lambda current: catalog(t),
        'rules.json': lambda current: rules(t),
        'journey_rules.json': lambda current: journey(t),
        'rogue_rules.json': lambda current: rogue(t),
        'draw_rules.json': lambda current: draw(t),
        'progression_events.json': lambda current: events(t),
        'system_rules.json': lambda current: system(t),
        'gm_rules.json': lambda current: gm(t, current),
        'network_rules.json': lambda current: network(t),
        'retail_pvp_rules.json': lambda current: retail_pvp(t),
        'retail_activity_rules.json': lambda current: retail_activity(t),
        'gameplay_defaults.json': lambda current: {'retail': retail(t)},
        'constant_rules.json': lambda current: constants(t),
    }
    report = {}
    try:
        for name, export in exporters.items():
            path = folder / name
            raw = path.read_text(encoding='utf-8') if path.exists() else ''
            current = json.loads(raw) if raw else {}
            result = dict(current)
            for key, value in export(current).items():
                merged = merge_section(name, key, current.get(key, {}), value)
                if merged != current.get(key):
                    report[name + ':' + key] = changes(current.get(key), merged)
                result[key] = merged
            if not arguments.check:
                write(path, result, raw)
    finally:
        tools.close()
    print(json.dumps(report, indent=1))


def main():
    parser = argparse.ArgumentParser(description='Export the server data tables in assets/datatables from the installed client tables.')
    parser.add_argument('--repo', type=Path, default=REPO)
    parser.add_argument('--analysis', type=Path, required=True, help='client analysis folder with the installed index and bytecode')
    parser.add_argument('--toolchain', type=Path, required=True, help='offline Lua toolchain folder with xlua.offline.dll')
    parser.add_argument('--check', action='store_true', help='report changes without writing assets')
    build(parser.parse_args())


if __name__ == '__main__':
    main()
