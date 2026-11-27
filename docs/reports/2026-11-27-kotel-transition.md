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

## Signed delivery

The release workflow creates a draft. Five native runners exercise the previous release's signed-channel parser against its candidate. Windows and Linux run the previous updater's replacement procedure. The Mac check reads its Sparkle feed, verifies the archives and the app's code signature, then opens the existing queue with the new binary. The queue comparison preserves all previous events and permits the additional pause event recorded at startup. This check was rehearsed with published Mac builds 114 and 115.

The signed-draft gate also makes a real mailbox update request through a loopback broker. On Linux the old helper must install the cached signed draft, restart the client and report the new version with the same pairing. On Mac it must return the native-window instruction.

Native Sparkle installation from the public bridge, systemd installation on a production server and deployment to individual computers are separate observations. A downloaded archive or queued remote operation does not count as a completed fleet installation. Keep the original repository name through that inventory.
