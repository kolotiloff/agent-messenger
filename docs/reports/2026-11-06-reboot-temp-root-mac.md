# The window would not open after a reboot: reservations in a deleted temporary directory

2026-11-06, the owner's Mac, installed preview.19.

## What happened

The Mac froze and was restarted. After that, the window showed:

> не удалось выбрать сохранённые данные: rejected: environment must be an existing directory
> ("could not choose the saved data")

`active-runs.json` held 7 reservations, all for `/private/tmp/am-claude-nee2aviw`:

- 4 were from one-shot runs on Nov 5. They have no recovery epoch, so they were recorded before epochs existed.
- 3 were from persistent streams; the last one is from the preview.19 acceptance run.

macOS removed that temporary directory on reboot. The window adopts the reservation's directory, and it refuses a missing one, so it could not start. No client could start in any other directory either.

**Root cause.** Acceptance runs used a data directory in `/private/tmp`. The window then adopted it, because it follows the protective registry. Because of that, the window's history lived in a folder that macOS empties on reboot.

## How it was resolved

The owner checked on the Mac:

- `ls -ld /private/tmp/am-claude-nee2aviw` reported "No such file or directory".
- `kern.boottime` showed today, 15:39.
- `ps` for the 7 recorded PIDs found only PID 626, which was an unrelated system process started after that boot.

The owner then moved `active-runs.json` aside, to `active-runs.json.before-reboot-2026-11-06`, without deleting it. The window started with the default data directory and empty history. This was a manual decision by the owner based on that evidence; no tool released the reservations automatically. The history that lived in the temporary directory is lost.

## Changes made

- **`messenger recover-missing-root --confirm preserve-uncertain`** retires registry entries whose data directory no longer exists. It does so only with a recorded epoch and a proven new boot of the same host. It would have handled the 3 stream reservations, but not the 4 legacy ones. The window error now names this command.
- **The window refuses a temporary directory.** It no longer adopts a reservation's data directory in a system temporary folder, and it says why.
- **New rule in `AGENTS.md`.** Acceptance runs must use a directory under `~/AgentMessengerTest/`, not a temporary folder.

These changes ship with the next app build. preview.19 does not have them.
