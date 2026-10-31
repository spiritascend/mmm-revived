# MMM Revived

Private server for Marvel Mystic Mayhem.

## You need

- [Go 1.27.2 or newer](https://go.dev/dl/)
- [just](https://github.com/casey/just#installation)
- [The Marvel MM CN game client](https://archive.org/details/marvel-mystic-mayhem-v-3-2262-full-build)

## Host a game

1. (Optional DLC) Download `MMM-2.1.20.zip` from https://archive.org/details/mmm-2.1.20
2. Put the zip in the `content` folder of this project (do not unzip it).
3. Close the game, then run:

   ```
   just configure "<game folder>"
   just run
   ```

4. Open the game and accept the download and restart prompts.

To let friends join, set `host` in `build/config.toml` to an address they can reach (a Tailscale IP works well), run `just configure "<game folder>"` again, then `just run`. Without a VPN, open TCP ports 14100 and 18080.

## Join a friend

You do not need the content zip.

1. Clone the Repo
2. run (in the project directory):

   ```
   just join "<game folder>" http://<host address>:18080
   ```

3. Open the game and accept the download and restart prompts.

## Updating

When a new version of this project comes out:

1. Close the game and stop the server (Ctrl+C).
2. In the project directory, run:

   ```
   git pull
   just configure "<game folder>"
   just run
   ```

3. Open the game and accept the download and restart prompts.

Friends who joined your server do not need to run anything: they get the update the next time they open the game. If their game stops connecting, they can run `git pull` and their `just join` command again.

## Config

Edit `build/config.toml`, then run `just configure "<game folder>"` and `just run` again.

| Option | What it does |
| --- | --- |
| `host` | Your IP that you or your friends connect to |
| `port` | Game port |
| `httpPort` | Download port (DLC) |
| `fresh_start` | `true` = start from zero // `false` = every item and cosmetic unlocked |
| `story_complete` | `true` = story and tutorials already finished at max team level (used when `fresh_start = false`) |
| `infinite_energy` | `true` = energy refills to max after every use (rewards and team XP unchanged) |
| `free_draws` | `true` = draws cost no keys (keeps 10 of each key) |
| `dreamstone_shop` | `true` = show the Dreamstone top-up page in the shop (top-ups are free) |
| `all_banners` | `true` = open every limited banner |
| `featured_banners` | Limited banners to open instead of the calendar rotation, e.g. `[5, 12]` (see below) |
| `calendar_loop` | `true` (default) = replay the dated content (Battle Pass, events, banners, PvP, guild wars) in a 231-day loop so it never runs out // `false` = real dates only |
| `all_events` | `true` (default) = every event stays open; one-at-a-time events (PvP seasons, Infinite Mayhem, bosses) rotate with no gaps // `false` = calendar dates only |
| `team_size` | Max heroes per battle (default `3`) |
| `energy_regen` | Energy regen speed, e.g. `1.25` = 25% faster (default `1`) |
| `reward_multiplier` | Multiplies Treasure Hunt, Rune Raid, Fabled Finds and other challenge rewards; XP stays the same (default `1`) |
| `all_cosmetics` | `true` = fresh-start accounts own every avatar, frame, background, badge and outfit (default `false`) |

Banner ids: 2 Iron Man, 3 Captain America, 4 Loki, 5 Invisible Woman, 6 Mister Fantastic, 7 Spider-Man, 8 Thor, 9 Gambit, 10 Juggernaut, 11 Namor, 12 Emma Frost, 13 Silver Surfer, 14 Doctor Strange, 15 Green Goblin, 17 Daredevil, 19 Magneto, 20 Storm, 21 Phoenix, 23 Venom, 25 Wolverine, 27 Cyclops, 29 Morgan le Fay. These options work with `fresh_start` and apply to existing accounts on the next login.

## What's added (Custom Content)

- **English translation** of the CN client: text, login screen and UI.
- **Friend duels:** challenge a friend from the friend list with Own Heroes, Random 5 or Current Draft, then pick how abilities are played:
  - **Auto Abilities:** heroes play themselves.
  - **Manual Abilities:** a custom PvP mode where you play every card and ultimate yourself, synced live between both players. No timer and no speed-up; the duel ends when one side's heroes are all dead.
- **Helper chat artifact command:** open the built-in **Helper** contact in the friend chat and send `/gain_artifact`. The private wizard lists available artifact IDs, then guides you through rarity, starting stats, upgrade rolls, and inventory quantity. Use `/cancel` to stop. The command does not use or alter world-chat rate limits.
- **Helper chat settings commands:** send `/commands` in Helper to see commands for artifact names and settings such as energy, draws, banners, and team size. Settings are saved to the sending player's account, apply immediately, and do not change other players' settings or the server's `build/config.toml`. `/featured_banners` accepts comma-separated banner IDs from the list above, or `none` to clear the player's featured-banner selection.
- **Live friend status:** online and offline updates show up instantly.
- **Latest CN content (2.1.20):** all heroes, events and missions.
- **Local login:** username and password, no NetEase account needed.

## Helper chat commands

Open the built-in **Helper** contact in the friend chat. Commands sent there are private; world chat and its rate limits are unchanged. `/commands` lists the commands in-game.

| Command | Usage |
| --- | --- |
| `/gain_artifact` | Start the guided artifact creation wizard. |
| `/artifact_ids` | List available artifact IDs and names. |
| `/cancel` | Cancel the active artifact wizard. |
| `/infinite_energy` | `/infinite_energy on` or `/infinite_energy off` (also accepts `true`/`false`, `yes`/`no`, or `1`/`0`; the misspelled `/infinte_energy` is also accepted). |
| `/free_draws` | `/free_draws on` or `/free_draws off` (also accepts `true`/`false`, `yes`/`no`, or `1`/`0`). |
| `/dreamstone_shop` | `/dreamstone_shop on` or `/dreamstone_shop off` (also accepts `true`/`false`, `yes`/`no`, or `1`/`0`). |
| `/all_banners` | `/all_banners on` or `/all_banners off` (also accepts `true`/`false`, `yes`/`no`, or `1`/`0`). |
| `/featured_banners` | `/featured_banners 2,5,12` using banner IDs listed in [Config](#config), or `/featured_banners none` to clear the selection. |
| `/team_size` | `/team_size 1` through `/team_size 5`. |
| `/commands` | Show the in-game command list. |

Helper settings are per-player account overrides of the server's defaults. They persist across logins and apply immediately to the player who sent the command; they do not edit `build/config.toml` or affect other players. Server owners can still use the Config options above to set defaults for all accounts.

## Issues

If you run into a problem, feel free to open an issue and I will get to it in my free time.
