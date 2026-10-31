import argparse
import json
import random
import sqlite3
import sys
import time
from pathlib import Path


ROOT = Path(__file__).resolve().parent
DEFAULT_UID = 100001
DEFAULT_DATABASE = ROOT / "build" / "data" / "accounts" / "mmm-revived.db"
TABLES = ROOT / "assets" / "datatables"
COMBAT_SIMULATOR_NAME = "Combat Simulator"
MAX_SIMULATOR_GRANT = 9999
MAX_ARTIFACT_COPIES = 10
EXCLUDED_BASE_IDS = {123, 124, 125, 126, 127, 151, 152, 153, 154}
ARTIFACT_NAMES = {
    101: "Asgard Shield",
    102: "Unstable Molecules",
    103: "Force Field Glove",
    104: "Gamma Mutagen",
    105: "Avengers ID Card",
    106: "Undrjarn",
    107: "Extremis",
    108: "The Hand's Shuriken",
    109: "Midnight Gun",
    110: "S.H.I.E.L.D. Flight Pack",
    111: "Sling Ring",
    112: "Stormbreaker",
    113: "Gungnir",
    114: "Darkhold",
    115: "Eye of Agamotto",
    116: "Cosmic Cube",
    117: "Crimson Gem",
    118: "La Fleurdu Mal",
    119: "Wand of Watoomb",
    120: "Evil Eye",
    121: "Storm Caster",
    122: "Crown of Blindness",
}


def load_json(path):
    with path.open(encoding="utf-8") as source:
        return json.load(source)


def artifact_id_list(catalog, rules):
    return sorted(
        artifact_id
        for artifact_id in catalog["artifacts"]
        if artifact_id % 1000 not in EXCLUDED_BASE_IDS
        and str(artifact_id) in rules["artifacts"]
    )


def valid_attributes(template):
    return {
        name: limits
        for name, limits in template["attributes"].items()
        if template["weights"].get(name, 0) > 0
        and len(limits) == 2
        and 0 <= limits[0] <= limits[1] <= 1_000_000_000
    }


def list_artifacts(catalog, rules):
    print(f"{'#':>2}  {'ID':>4}  {'Rarity':>6}  {'Name':<25}  Template(s)")
    print("-" * 66)
    for index, artifact_id in enumerate(artifact_id_list(catalog, rules), 1):
        rule = rules["artifacts"][str(artifact_id)]
        name = ARTIFACT_NAMES.get(artifact_id % 1000, "Artifact")
        templates = ", ".join(str(value) for value in rule["template"])
        print(f"{index:>2}  {artifact_id:>4}  {rule['star']:>6}  {name:<25}  {templates}")


def roll_stats(template, count):
    attributes = valid_attributes(template)
    weights = template["weights"]
    available = list(attributes)
    if count > len(available):
        raise ValueError("template has too few valid attributes for this rarity")

    growth = {}
    order = []
    for _ in range(count):
        name = random.choices(
            available,
            weights=[weights[name] for name in available],
            k=1,
        )[0]
        available.remove(name)
        growth[name] = [random.randint(*attributes[name])]
        order.append(name)
    return growth, order, {name: values[0] for name, values in growth.items()}


def custom_stats(template, count, entries):
    attributes = valid_attributes(template)
    if len(entries) != count:
        raise ValueError(
            f"this artifact requires exactly {count} initial stats; "
            f"received {len(entries)}"
        )

    growth = {}
    order = []
    for entry in entries:
        if "=" not in entry:
            raise ValueError(f"invalid stat {entry!r}; expected name=value")
        name, text_value = entry.split("=", 1)
        name = name.strip()
        if name not in attributes:
            raise ValueError(f"stat {name!r} is not supported by this template")
        low, high = attributes[name]
        try:
            value = int(text_value)
        except ValueError as error:
            raise ValueError(f"stat value must be an integer: {entry!r}") from error
        if not low <= value <= high:
            raise ValueError(
                f"{name} must be between {low} and {high} for this template"
            )
        growth.setdefault(name, []).append(value)
        if name not in order:
            order.append(name)

    return growth, order, {
        name: sum(values) for name, values in growth.items()
    }


def custom_rolls(template, growth, entries, max_attributes):
    attributes = valid_attributes(template)
    growth = {name: values.copy() for name, values in growth.items()}
    order = list(growth)
    for index, entry in enumerate(entries, 1):
        if "=" not in entry:
            raise ValueError(f"invalid stat roll {entry!r}; expected name=value")
        name, text_value = entry.split("=", 1)
        name = name.strip()
        if name not in attributes:
            raise ValueError(f"stat {name!r} is not supported by this template")
        if len(growth) < max_attributes and name in growth:
            raise ValueError(
                f"upgrade roll {index} must add a new stat until the artifact "
                f"has {max_attributes} different stats"
            )
        if len(growth) >= max_attributes and name not in growth:
            raise ValueError(
                f"upgrade roll {index} must repeat an existing stat once the "
                f"artifact has {max_attributes} different stats"
            )
        low, high = attributes[name]
        try:
            value = int(text_value)
        except ValueError as error:
            raise ValueError(
                f"stat value must be an integer: {entry!r}"
            ) from error
        if not low <= value <= high:
            raise ValueError(
                f"{name} must be between {low} and {high} for this template"
            )
        growth.setdefault(name, []).append(value)
        if name not in order:
            order.append(name)

    return growth, order, {
        name: sum(values) for name, values in growth.items()
    }


def prompt_stats(template, count):
    growth = {}
    order = []
    attributes = valid_attributes(template)

    print(f"\nChoose {count} starting stat rolls. Stats can repeat.")
    for slot in range(count):
        available = list(attributes)
        print(f"\nStat {slot + 1} of {count}:")
        for index, name in enumerate(available, 1):
            low, high = attributes[name]
            print(f"  {index}) {name} ({low}..{high})")
        while True:
            choice = input("Choose stat [number/name]: ").strip()
            if choice.isdecimal() and 1 <= int(choice) <= len(available):
                name = available[int(choice) - 1]
                break
            if choice in attributes:
                name = choice
                break
            print("Choose one of the listed numbers or stat names.")

        low, high = attributes[name]
        while True:
            text_value = input(
                f"  Value for {name} ({low}..{high}): "
            ).strip()
            try:
                value = int(text_value)
            except ValueError:
                print("Enter an integer.")
                continue
            if low <= value <= high:
                break
            print(f"Value must be between {low} and {high}.")

        growth.setdefault(name, []).append(value)
        if name not in order:
            order.append(name)

    return growth, order, {
        name: sum(values) for name, values in growth.items()
    }


def prompt_extra_rolls(template, growth, max_rolls, max_attributes):
    while True:
        value = input(
            f"How many upgrade stat rolls to configure? (0-{max_rolls}) [0]: "
        ).strip()
        if not value:
            return custom_rolls(template, growth, [], max_attributes), 0
        if value.isdecimal() and int(value) <= max_rolls:
            count = int(value)
            break
        print(f"Enter a whole number from 0 to {max_rolls}.")

    attributes = valid_attributes(template)
    entries = []
    current_attributes = set(growth)
    for slot in range(count):
        print(f"\nUpgrade roll {slot + 1} of {count}:")
        if len(current_attributes) < max_attributes:
            choices = {
                name: limits
                for name, limits in attributes.items()
                if name not in current_attributes
            }
        else:
            choices = {
                name: attributes[name]
                for name in current_attributes
            }
        if not choices:
            raise ValueError("no valid stat choices are available for this upgrade")
        for index, (name, limits) in enumerate(choices.items(), 1):
            print(f"  {index}) {name} ({limits[0]}..{limits[1]})")
        while True:
            choice = input("Choose stat [number/name]: ").strip()
            if choice.isdecimal() and 1 <= int(choice) <= len(choices):
                name = list(choices)[int(choice) - 1]
                break
            if choice in choices:
                name = choice
                break
            print("Choose one of the listed numbers or stat names.")

        low, high = choices[name]
        while True:
            text_value = input(f"  Value for {name} ({low}..{high}): ").strip()
            try:
                stat_value = int(text_value)
            except ValueError:
                print("Enter an integer.")
                continue
            if low <= stat_value <= high:
                entries.append(f"{name}={stat_value}")
                current_attributes.add(name)
                break
            print(f"Value must be between {low} and {high}.")
    return custom_rolls(template, growth, entries, max_attributes), count


def choose_artifact(available):
    artifact_ids = sorted(available)
    while True:
        choice = input("Choose by # or artifact ID: ").strip()
        if choice.isdecimal() and 1 <= int(choice) <= len(artifact_ids):
            return artifact_ids[int(choice) - 1]
        artifact_id = int(choice) if choice.isdecimal() else 0
        if artifact_id in available:
            return artifact_id
        print("Choose a listed number or artifact ID.")


def choose_template(template_ids):
    if len(template_ids) == 1:
        return template_ids[0]
    print("\nAvailable templates:")
    for index, template_id in enumerate(template_ids, 1):
        print(f"  {index}) Template {template_id}")
    while True:
        choice = input("Choose template [number/ID]: ").strip()
        if choice.isdecimal():
            value = int(choice)
            if 1 <= value <= len(template_ids):
                return template_ids[value - 1]
            if value in template_ids:
                return value
        print("Choose one of the listed numbers or template IDs.")


def confirm(prompt):
    return input(f"{prompt} [y/N]: ").strip().lower() in ("y", "yes")


def ensure_database(path):
    if not path.is_file():
        raise ValueError(
            f"database not found: {path}\n"
            "Run this tool from the repository containing build\\data\\accounts, "
            "or pass the correct --database path."
        )


def prompt_simulator_quantity():
    while True:
        value = input(
            f"How many {COMBAT_SIMULATOR_NAME}s to add? (1-{MAX_SIMULATOR_GRANT}): "
        ).strip()
        if value.isdecimal() and 1 <= int(value) <= MAX_SIMULATOR_GRANT:
            return int(value)
        print(f"Enter a whole number from 1 to {MAX_SIMULATOR_GRANT}.")


def prompt_artifact_copies():
    while True:
        value = input(
            f"How many copies of this artifact? (1-{MAX_ARTIFACT_COPIES}): "
        ).strip()
        if value.isdecimal() and 1 <= int(value) <= MAX_ARTIFACT_COPIES:
            return int(value)
        print(f"Enter a whole number from 1 to {MAX_ARTIFACT_COPIES}.")


def prompt_stat_count(initial_count, max_count):
    if initial_count == max_count:
        return initial_count
    while True:
        value = input(
            f"How many starting stats? ({initial_count}-{max_count}) "
            f"[{initial_count}]: "
        ).strip()
        if not value:
            return initial_count
        if value.isdecimal() and initial_count <= int(value) <= max_count:
            return int(value)
        print(f"Enter a whole number from {initial_count} to {max_count}.")


def add_simulators(args, constants):
    ensure_database(args.database)
    item_id = constants["sweep_challenge_item"]
    print(f"\nAdd {COMBAT_SIMULATOR_NAME}")
    print(f"  Account: {args.uid}")
    print(f"  Item ID: {item_id}")
    amount = prompt_simulator_quantity()

    if args.dry_run:
        print(f"Dry run: would add {amount} {COMBAT_SIMULATOR_NAME}(s).")
        return
    print(f"Database: {args.database}")
    print("Stop the game server before writing to avoid stale account data.")
    if not args.yes and not confirm(f"Add {amount} {COMBAT_SIMULATOR_NAME}(s)"):
        print("Cancelled; no database changes made.")
        return

    db = sqlite3.connect(args.database)
    try:
        db.execute("BEGIN IMMEDIATE")
        if db.execute(
            "SELECT 1 FROM accounts WHERE uid = ?", (args.uid,)
        ).fetchone() is None:
            raise ValueError(f"account {args.uid} does not exist")

        row = db.execute(
            "SELECT payload FROM account_records "
            "WHERE uid = ? AND category = 'item' AND id = ?",
            (args.uid, str(item_id)),
        ).fetchone()
        current = 0 if row is None else json.loads(row[0])
        if not isinstance(current, int) or isinstance(current, bool) or current < 0:
            raise ValueError(
                f"stored {COMBAT_SIMULATOR_NAME} quantity is invalid: {current!r}"
            )
        updated = current + amount
        if updated > 9_223_372_036_854_775_807:
            raise ValueError(f"{COMBAT_SIMULATOR_NAME} quantity would overflow")

        db.execute(
            "INSERT INTO account_records(uid, category, id, payload) "
            "VALUES (?, 'item', ?, ?) "
            "ON CONFLICT(uid, category, id) DO UPDATE SET payload=excluded.payload",
            (
                args.uid,
                str(item_id),
                json.dumps(updated, separators=(",", ":")),
            ),
        )
        db.commit()
        print(
            f"Added {amount} {COMBAT_SIMULATOR_NAME}(s). "
            f"New inventory total: {updated}."
        )
        print("Restart the server and log the account back in to see the change.")
    except Exception:
        db.rollback()
        raise
    finally:
        db.close()


def add_artifact(args, catalog, rules, constants):
    available_ids = artifact_id_list(catalog, rules)
    if not available_ids:
        raise ValueError("the rules contain no available artifacts")

    if args.artifact is None:
        list_artifacts(catalog, rules)
        artifact_id = choose_artifact(set(available_ids))
    else:
        artifact_id = args.artifact
        if artifact_id not in available_ids:
            raise ValueError(
                f"artifact {artifact_id} is unavailable or is on the excluded list"
            )

    rule = rules["artifacts"][str(artifact_id)]
    if args.template is None:
        template_id = choose_template(rule["template"])
    else:
        template_id = args.template
    if template_id not in rule["template"]:
        raise ValueError(
            f"template {template_id} is not valid for artifact {artifact_id}; "
            f"choose from {rule['template']}"
        )

    template = rules["artifact_templates"].get(str(template_id))
    if template is None:
        raise ValueError(f"artifact template {template_id} is missing from rules")
    initial_count = constants["artifact_rarity_subattr_num"].get(str(rule["star"]))
    if initial_count is None:
        raise ValueError(f"no initial stat count configured for rarity {rule['star']}")
    max_count = constants["artifact_rarity_subattr_num_max"].get(
        str(rule["star"]), initial_count
    )

    if args.stat:
        count = len(args.stat)
        if not initial_count <= count <= max_count:
            raise ValueError(
                f"this artifact supports {initial_count} to {max_count} starting "
                f"stats; received {count}"
            )
        growth, order, sub = custom_stats(template, count, args.stat)
    elif args.artifact is None:
        count = prompt_stat_count(initial_count, max_count)
        growth, order, sub = prompt_stats(template, count)
    else:
        raise ValueError(
            "provide starting stats with --stat when using --artifact"
        )

    sub_levels = rule.get("sub_levels", [])
    max_upgrade_rolls = len(sub_levels)
    if args.roll:
        if len(args.roll) > max_upgrade_rolls:
            raise ValueError(
                f"artifact supports at most {max_upgrade_rolls} configured "
                f"upgrade rolls; received {len(args.roll)}"
            )
        growth, order, sub = custom_rolls(
            template, growth, args.roll, max_count
        )
        upgrade_roll_count = len(args.roll)
    elif args.artifact is None:
        (growth, order, sub), upgrade_roll_count = prompt_extra_rolls(
            template, growth, max_upgrade_rolls, max_count
        )
    else:
        upgrade_roll_count = 0
    level = sub_levels[upgrade_roll_count - 1] if upgrade_roll_count else 1

    payload = {
        "Growth": growth,
        "PendingGrowth": {},
        "Order": order,
        "PendingOrder": None,
        "Install": 0,
        "Template": template_id,
        "ID": artifact_id,
        "Level": level,
        "Experience": 0,
        "Locked": 0,
        "Sub": sub,
        "Obtained": int(time.time()),
    }
    print(f"\nArtifact preview")
    print(f"  Account:  {args.uid}")
    print(f"  Artifact: {artifact_id} - {ARTIFACT_NAMES.get(artifact_id % 1000, 'Artifact')}")
    print(f"  Rarity:   {rule['star']}")
    print(f"  Template: {template_id}")
    print(f"  Level:    {level}")
    print(
        "Stats:",
        ", ".join(
            f"{name}={' + '.join(str(value) for value in values)}"
            for name, values in growth.items()
        ),
    )

    if args.dry_run:
        print("Dry run: no database changes made.")
        return
    ensure_database(args.database)
    print(f"Database: {args.database}")
    print("Stop the game server before writing to avoid stale account data.")
    if not args.yes and not confirm("Add this artifact"):
        print("Cancelled; no database changes made.")
        return
    copies = 1 if args.yes else prompt_artifact_copies()

    db = sqlite3.connect(args.database)
    try:
        db.execute("BEGIN IMMEDIATE")
        row = db.execute(
            "SELECT profile FROM accounts WHERE uid = ?", (args.uid,)
        ).fetchone()
        if row is None:
            raise ValueError(f"account {args.uid} does not exist")

        profile = json.loads(row[0])
        used_ids = {
            instance_id
            for (instance_id,) in db.execute(
                "SELECT id FROM account_records "
                "WHERE uid = ? AND category IN ('artifact', 'highlight')",
                (args.uid,),
            )
        }
        next_instance = max(
            profile.get("next_instance", 0),
            max(
                (
                    int(instance_id)
                    for instance_id in used_ids
                    if instance_id.isdigit()
                ),
                default=0,
            ),
        ) + 1
        while str(next_instance) in used_ids:
            next_instance += 1

        new_rows = []
        for _ in range(copies):
            while str(next_instance) in used_ids:
                next_instance += 1
            new_rows.append(
                (
                    args.uid,
                    str(next_instance),
                    json.dumps(payload, separators=(",", ":")),
                )
            )
            used_ids.add(str(next_instance))
            next_instance += 1

        db.executemany(
            "INSERT INTO account_records(uid, category, id, payload) "
            "VALUES (?, 'artifact', ?, ?)",
            new_rows,
        )
        profile["next_instance"] = next_instance - 1
        db.execute(
            "UPDATE accounts SET profile = ? WHERE uid = ?",
            (json.dumps(profile, separators=(",", ":")), args.uid),
        )
        db.commit()
        print(f"Added {copies} artifact copy/copies to account {args.uid}.")
        print("Restart the server and log the account back in to see it.")
    except Exception:
        db.rollback()
        raise
    finally:
        db.close()


def bulk_add(args, catalog, rules, constants):
    initial_attributes = constants["artifact_rarity_subattr_num"]
    ensure_database(args.database)
    db = sqlite3.connect(args.database)
    try:
        db.execute("BEGIN IMMEDIATE")
        row = db.execute(
            "SELECT profile FROM accounts WHERE uid = ?", (args.uid,)
        ).fetchone()
        if row is None:
            raise ValueError(f"account {args.uid} does not exist")
        profile = json.loads(row[0])
        records = db.execute(
            "SELECT id, payload FROM account_records "
            "WHERE uid = ? AND category = 'artifact'",
            (args.uid,),
        ).fetchall()
        counts = {}
        for _, payload_data in records:
            artifact_id = json.loads(payload_data)["ID"]
            counts[artifact_id] = counts.get(artifact_id, 0) + 1

        used_ids = {
            instance_id
            for (instance_id,) in db.execute(
                "SELECT id FROM account_records "
                "WHERE uid = ? AND category IN ('artifact', 'highlight')",
                (args.uid,),
            )
        }
        next_instance = max(
            profile.get("next_instance", 0),
            max(
                (
                    int(instance_id)
                    for instance_id in used_ids
                    if instance_id.isdigit()
                ),
                default=0,
            ),
        )
        new_rows = []
        for artifact_id in artifact_id_list(catalog, rules):
            artifact = rules["artifacts"][str(artifact_id)]
            missing = max(0, args.copies - counts.get(artifact_id, 0))
            for _ in range(missing):
                template_id = random.choice(artifact["template"])
                template = rules["artifact_templates"][str(template_id)]
                growth, order, sub = roll_stats(
                    template,
                    initial_attributes[str(artifact["star"])],
                )
                next_instance += 1
                while str(next_instance) in used_ids:
                    next_instance += 1
                instance_id = str(next_instance)
                used_ids.add(instance_id)
                payload = {
                    "Growth": growth,
                    "PendingGrowth": {},
                    "Order": order,
                    "PendingOrder": None,
                    "Install": 0,
                    "Template": template_id,
                    "ID": artifact_id,
                    "Level": 1,
                    "Experience": 0,
                    "Locked": 0,
                    "Sub": sub,
                    "Obtained": int(time.time()),
                }
                new_rows.append(
                    (
                        args.uid,
                        instance_id,
                        json.dumps(payload, separators=(",", ":")),
                    )
                )
                counts[artifact_id] = counts.get(artifact_id, 0) + 1

        print(
            f"Account {args.uid}: {len(artifact_id_list(catalog, rules))} artifact types; "
            f"{len(new_rows)} new instances needed to reach {args.copies} each."
        )
        if args.dry_run:
            db.rollback()
            print("Dry run: no database changes made.")
            return
        print(f"Database: {args.database}")
        print("Stop the game server before writing to avoid stale account data.")
        if not args.yes and not confirm("Add these artifacts"):
            db.rollback()
            print("Cancelled; no database changes made.")
            return

        db.executemany(
            "INSERT INTO account_records(uid, category, id, payload) "
            "VALUES (?, 'artifact', ?, ?)",
            new_rows,
        )
        profile["next_instance"] = next_instance
        db.execute(
            "UPDATE accounts SET profile = ? WHERE uid = ?",
            (json.dumps(profile, separators=(",", ":")), args.uid),
        )
        db.commit()
        print(f"Added {len(new_rows)} artifact instances to account {args.uid}.")
        print("Restart the server and log the account back in to see them.")
    except Exception:
        db.rollback()
        raise
    finally:
        db.close()


def interactive_menu(catalog, rules, constants):
    print("Artifact Inventory Tool")
    print(f"Account: {DEFAULT_UID}    Database: {DEFAULT_DATABASE}")
    while True:
        print("\nWhat would you like to do?")
        print("  1) Add artifacts with custom stats")
        print("  2) Add copies of all available artifacts")
        print(f"  3) Add {COMBAT_SIMULATOR_NAME}s")
        print("  4) List available artifacts")
        print("  5) Exit")
        choice = input("Choose an option [1]: ").strip() or "1"
        try:
            if choice == "1":
                args = argparse.Namespace(
                    uid=DEFAULT_UID,
                    database=DEFAULT_DATABASE,
                    artifact=None,
                    template=None,
                    stat=None,
                    roll=None,
                    dry_run=False,
                    yes=False,
                )
                add_artifact(args, catalog, rules, constants)
            elif choice == "2":
                copies_text = input("Copies of each artifact [3]: ").strip() or "3"
                if not copies_text.isdecimal() or int(copies_text) < 1:
                    print("Enter a whole number greater than zero.")
                    continue
                args = argparse.Namespace(
                    uid=DEFAULT_UID,
                    database=DEFAULT_DATABASE,
                    copies=int(copies_text),
                    dry_run=False,
                    yes=False,
                )
                bulk_add(args, catalog, rules, constants)
            elif choice == "3":
                args = argparse.Namespace(
                    uid=DEFAULT_UID,
                    database=DEFAULT_DATABASE,
                    dry_run=False,
                    yes=False,
                )
                add_simulators(args, constants)
            elif choice == "4":
                list_artifacts(catalog, rules)
            elif choice == "5":
                print("Goodbye.")
                return
            else:
                print("Choose 1, 2, 3, 4, or 5.")
        except ValueError as error:
            print(f"\nError: {error}")
        except sqlite3.Error as error:
            print(f"\nDatabase error: {error}")
            print("Check the database path and make sure the server is stopped.")


def main():
    parser = argparse.ArgumentParser(
        description="Add valid artifacts with custom stats to an account inventory."
    )
    commands = parser.add_subparsers(dest="command", required=True)

    commands.add_parser("list", help="list available artifact IDs and templates")

    add = commands.add_parser("add", help="add one artifact")
    add.add_argument("--uid", type=int, default=DEFAULT_UID)
    add.add_argument("--database", type=Path, default=DEFAULT_DATABASE)
    add.add_argument("--artifact", type=int, help="full artifact ID, e.g. 5101")
    add.add_argument("--template", type=int, help="valid template ID for the artifact")
    add.add_argument(
        "--stat",
        action="append",
        help="set a starting stat roll as name=value; repeat once per starting roll",
    )
    add.add_argument(
        "--roll",
        action="append",
        help="set an upgrade stat roll as name=value; repeat to configure more rolls",
    )
    add.add_argument("--dry-run", action="store_true")
    add.add_argument("--yes", action="store_true", help="skip the write confirmation")

    bulk = commands.add_parser(
        "bulk", help="add enough copies to reach a specified count of each artifact"
    )
    bulk.add_argument("--uid", type=int, default=DEFAULT_UID)
    bulk.add_argument("--database", type=Path, default=DEFAULT_DATABASE)
    bulk.add_argument("--copies", type=int, default=3)
    bulk.add_argument("--dry-run", action="store_true")
    bulk.add_argument("--yes", action="store_true", help="skip the write confirmation")

    simulators = commands.add_parser(
        "add-simulators",
        help=f"add {COMBAT_SIMULATOR_NAME} after entering a quantity",
    )
    simulators.add_argument("--uid", type=int, default=DEFAULT_UID)
    simulators.add_argument("--database", type=Path, default=DEFAULT_DATABASE)
    simulators.add_argument("--dry-run", action="store_true")
    simulators.add_argument("--yes", action="store_true", help="skip the write confirmation")

    catalog = load_json(TABLES / "catalog.json")
    rules = load_json(TABLES / "rules.json")
    constants = load_json(TABLES / "constant_rules.json")["constants"]

    if len(sys.argv) == 1:
        interactive_menu(catalog, rules, constants)
        return

    args = parser.parse_args()
    try:
        if args.command == "list":
            list_artifacts(catalog, rules)
        elif args.command == "add":
            add_artifact(args, catalog, rules, constants)
        elif args.command == "bulk":
            if args.copies < 1:
                parser.error("--copies must be greater than zero")
            bulk_add(args, catalog, rules, constants)
        elif args.command == "add-simulators":
            add_simulators(args, constants)
    except (ValueError, sqlite3.Error) as error:
        parser.error(str(error))


if __name__ == "__main__":
    main()
