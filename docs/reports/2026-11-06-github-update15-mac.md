# Mac preview.14 → preview.15 update

November 6, 2026, Mac Apple Silicon. Source commit `d4f821ae556779a9112346f41142d4b13fd53b9d`.

The package was obtained from the public GitHub Release through the built-in `update-fetch`. Observed: the offer of version 15, clicking «Установить обновление» ("Install update"), unpacking, and installed version 15 in the window. The archive and metadata were not copied into the active channel by hand.

After installation, all fields of the `inspect` snapshot matched the original, including 13 messages, 10 attempts and 9 replies. The background client's PID is unchanged, and model launches are disabled. No working sessions were started.

Separate process monitoring recorded an automatic `update-fetch` when version 15 started and a second one 302 seconds later. Between them, the verification tooling performed no manual check, restart or activation of Messenger. This confirms repeated checking by an already open application; it is not a check of push delivery or a guarantee of detecting a release within five minutes under all network conditions.

All four [CI](https://github.com/kolotiloff/agent-messenger/actions/runs/42183471895) jobs passed. In the browser test bench with demo data, the following were checked: light/dark themes, the scrollbar at the queue's edge, a fixed input and growth of the multiline field. After the release, the owner noticed asymmetric padding: fixing it belongs to preview.16.
