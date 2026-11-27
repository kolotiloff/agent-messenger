# Kotel bridge acceptance

The bridge is version 1.0.13, build 116. The original repository retains the public update channel during the fleet rollout. [Update and recovery procedure](../UPDATES.md#kotel-transition-build-116)

## Isolated macOS runs

| Starting build | Data after restarting into the local 116 candidate | Network compatibility |
| --- | --- | --- |
| 1.0.5, build 108 | Both projects, task histories, statistics totals and scoped exports matched. SQLite integrity checks passed after reopening the window. | Checked an old host with a new member and a new host with an old member. |
| 1.0.9, build 112 | Same checks passed. | Both combinations passed. |
| 1.0.12, build 115 | Same checks passed. | Both combinations passed. |

Two additional fixtures started from signed builds 99 and 101. The bridge kept their queued request and prior events without launching a model.

The fixtures used independent home directories under the Mac acceptance directory, loopback-only brokers and synthetic data. A single pairing survived successive client and host upgrades. Checks compared the device ID, broker ID and certificate fingerprint and required exactly one matching device record. In a separate loopback rehearsal the bridge host requested a remote update first from the old Mac and then from the bridge Mac. Each returned its own matching request ID, directed the owner to update in the Mac application and kept its device identity. No real provider was invoked.

The AppKit candidate showed Kotel, the first-run actions, separate Statistics and the Settings order General, Project, Agents, Server. The final pot icon has widely spaced eyes and the right edge of the lid raised. The Mac icon set includes 16 px through 1024 px. The Windows resource test compared pixels and both architectures' compiled resources.

## Older split project databases

The 1.0.12 Windows failure was reproduced by opening a selected generation with a running remote task after removing its two live-usage columns. Before the fix, opening its conversation failed with `no such column: remote_live_tokens`. The regression exercises neither column, each single column and both columns as after a manual repair. It reopens through the ordinary store twice, compares the task history and peer, checks retained values and defaults, then delivers a live usage message through the mailbox. New project files are also opened directly to check their schema. In separate end-to-end Mac runs, all four test projects created by stock 108 and 112 lacked both columns. The fixed bridge added both on first use. Two fresh 115 projects already contained them. Each version retained its two projects, histories, statistics and exports after restarting the client and reopening the window.

## Signed delivery

The release workflow creates a draft. Five native runners exercise the previous release's signed-channel parser against its candidate. Windows and Linux run the previous updater's replacement procedure. The Mac check reads its Sparkle feed, verifies the archives and the app's code signature, then opens the existing queue with the new binary. The queue comparison preserves all previous events and permits the additional pause event recorded at startup. This check was rehearsed with published Mac builds 114 and 115.

The signed-draft gate also makes a real mailbox update request through a loopback broker. On Linux the old helper must install the cached signed draft, restart the client and report the new version with the same pairing. On Mac it must return the native-window instruction.

Native Sparkle installation from the public bridge, systemd installation on a production server and deployment to individual computers are separate observations. A downloaded archive or queued remote operation does not count as a completed fleet installation. Keep the original repository name through that inventory.


## Publication

The [signed Kotel bridge](https://github.com/kolotiloff/agent-messenger/releases/tag/v1.0.13-preview.116) was published on November 27 at 13:58 UTC from `c095407bbdab5d9e83e2270b43e0f64ee2e9eace`. The Windows test harness was corrected to decode UTF-8; the original signed packages were used throughout. [All five signed installation checks and public channel checks passed](https://github.com/kolotiloff/agent-messenger/actions/runs/46011237589). Both Linux workers reported the new version through their private broker and retained their pairing. Stock Mac builds 99, 101, 108, 112 and 115 discovered the public release through the old repository name. Stock build 115 downloaded the production Mac ZIP. Its digest and native signature passed, and the Kotel binary reported the exact release revision and a current update status through the GitHub repository ID. Repeating the two-project data and restart checks with this public binary passed from 108, 112 and 115, including live-usage columns and database integrity. The owner will confirm actual installations before authorizing the original repository rename.
