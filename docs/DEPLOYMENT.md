# Broker placement: VPS or your own computer

Agreed principle, November 5, 2026: **one broker component and one protocol regardless of where it runs**. A VPS is optional. These are implementation requirements; the switch and the server are not implemented yet.

## Application modes

| Choice | What happens |
|---|---|
| Connect to a server | The user enters the broker address and registers the device. The broker can be on a VPS or another computer. |
| Run a server on this computer | The application starts a local broker as a separate background service and connects its own client to it. Agents can run on the same machine. |

For the first launch on a single Mac, a local broker reachable only from that machine is used. Access for other devices is enabled explicitly. Client mode does not start a fallback broker when the connection is lost.

On a VPS the same broker is installed as a separate server package with no GUI and without Claude/Codex/Cursor. Settings, database format, API and compatibility checks are shared; packaging and OS service integration differ.

## Settings and first launch

To connect: server address, device name, a one-time invitation or a device approval procedure, then "Check connection". After registration, a separate revocable device key is stored in local secure storage.

This is access to Messenger, not the root/SSH password of the VPS. A regular client does not need administrative privileges on the server. Agent credentials are not sent to the broker. Installing on a new VPS has a separate wizard with temporary SSH access; after installation the client uses a restricted Messenger key.

To host on your computer: "Enable server", optional autostart, data storage location, address/port and access scope. Access is local by default; LAN/VPN/internet are enabled explicitly. After setup, the application shows the connection address and creates short-lived one-time invitations. Invitations and keys are not shown in the regular message log.

The interface shows separately:
- whether the local broker service is running;
- which broker the client is connected to and whether it is reachable;
- the address for other devices and the result of the network reachability check;
- the state of the local client and agents;
- the location of the database and backups, and the latest service errors.

"Broker running" does not mean it is reachable from another computer. Connecting over the internet may require a VPN or network/router configuration. The program does not open ports and does not promise to get around NAT with a single switch. Remote connections go through TLS verification and authorization; hosting on a home PC does not waive these requirements.

## VPS setup wizard in the app

Addendum from November 5, 2026: the application must be able to automatically install the broker on a supported VPS as a separate user action. This is a future feature of the network stage, not an installer that already works. Connecting to an existing broker remains a separate simple scenario.

### User flow

1. Choose "VPS → Install broker".
2. Enter the server IP/name, SSH port and user. Choose an existing SSH key/ssh-agent, or a password if the server allows that method. For a protected key, the standard passphrase prompt is used.
3. Verify the server identity: on the first connection, show the SSH key fingerprint and how to check it via the provider's panel or another trusted channel. Do not disable key checking; an unexpected key change stops the installation.
4. Run diagnostics: supported OS/architecture, free space, installation privileges, available ports, existing installation and network access. Show a concrete list of changes and an "Install" button. If administrative rights are needed, the wizard explains why and uses the provided privilege escalation mechanism.
5. Transfer the verified package, create a dedicated service account and data directory, install the broker and the autostart service, configure the selected secure address. The broker runs without permanent root privileges.
6. Check the service, HTTPS/certificate trust, reachability from the application specifically, and device registration. Send a technical test message with storage confirmation, without running models.
7. Save the address, broker_id and the restricted device key. Show "Connected" only after the checks succeed.

For a public address, the wizard offers a domain and automatic certificate management. IP/VPN mode must have a separate verified TLS/trust method; simply disabling certificate verification is not allowed. The provider's external firewall may require a separate action in its panel: the wizard shows this as an unfinished step instead of promising to fix it with SSH access alone.

### Secrets and execution

SSH access is used by the local installer only for installation and selected administrative operations. The private key is not copied to the VPS; the password and passphrase are not saved by default. Saving them for later maintenance is possible only by user choice, via the OS secure storage. Agent forwarding is not required.

Access to the installed Messenger uses a separate revocable device key, without SSH/root privileges. Secrets do not end up in model requests, logged commands, telemetry or the repository. Installation is performed by ordinary code using versioned scripts, without a model involved.

A fixed-version package is checked for integrity and provenance. For a private GitHub repository, the application can download the package locally and transfer it over SSH: a GitHub token does not need to be stored on the VPS.

### Errors, re-running the installation, and supported scope

Visible states: "Checking access", "Action needed", "Installing", "Setting up secure connection", "Checking broker", "Connected", "Error". SSH reachability and Messenger health are shown separately.

Each step saves an anonymized result. After a disconnect, the wizard re-checks the actual server state before continuing. A re-run does not reset the database, keys or broker_id and does not start a second instance. Update and removal are separate from the first installation; deleting data requires a separate explicit choice.

The first version is limited to a published list of supported Linux systems and architectures, which will be chosen and verified during implementation. An unsupported OS, a busy port or someone else's configuration produce a clear refusal/manual step; the wizard does not overwrite existing sites and services and does not change the server's SSH settings automatically.

Acceptance: clean installation, re-running the wizard, losing SSH at an intermediate step, wrong privileges, SSH host key change, busy port, unreachable from the client, invalid TLS, and preservation of the existing queue. The automation is considered ready only after these experiments.

References for SSH checks and key handling: [OpenSSH host key checking](https://man.openbsd.org/ssh_config#StrictHostKeyChecking), [ssh-agent](https://man.openbsd.org/ssh-agent).

## Architecture contract

The broker, the local client and the UI are separate components. The UI controls the service through a restricted local interface. Only the local client launches agent processes. Closing the window must not silently stop the broker or lose the queue; stopping the service and quitting the application have explicit states.

The configuration distinguishes:
- the local broker launch settings;
- the client connection address;
- a stable `broker_id`, project/message space and protocol version;
- the device ID and a reference to its local key;
- bindings of local CLI sessions.

The address can change, but the address itself is not the broker's identity. Queues, delivery cursors and deduplication keys are bound to the `broker_id` and the project. Connecting to a different broker must not silently send it the old queue. On an identity or version mismatch, the client stops processing and shows the reason.

The same tests for delivery, ACK, duplicates, authorization and recovery are run for a broker on a computer and on a VPS. Data is stored outside the application directory and the source repository. The virtual server does not need a GPU: the broker does not run models.

## Roles and availability

In the MVP, one broker is active per message space. A computer's server role is unrelated to the coordinator agent role. The coordinator can be reassigned without moving the broker database.

When the broker machine is turned off or the network is lost, delivery through it pauses. Clients keep their queues, show offline and do not create a second owner of a CLI session. Processing that has already started follows the existing rules for saving the result locally. Disk loss requires restoring a backup; local queues do not replace backing up the full history.

Spontaneous election of another computer as broker, automatic replication and clustering are not part of the first MVP.

## Moving between your own computer and a VPS

The architecture must allow moving without changing the protocol, but this is a managed procedure, not switching between two simultaneously active copies.

1. Pause handing out new jobs, reconcile current executions and outgoing results.
2. Stop the old broker and take a consistent backup of data/metadata.
3. Restore on the new node, check compatibility, integrity, identity and permissions; configure the secure connection.
4. Make sure the old copy will not resume serving. Switch client addresses; reissue keys safely if needed.
5. Reconcile the queue, cursors and deduplication, then resume work.

CLI history stays on the agent computers. A detailed migration utility and its acceptance are a separate task; the first version does not promise migration without downtime.

## Acceptance stages

- Local version on Mac: the same broker on a local address plus the client, without an external VPS.
- Network stage: one computer as the broker, another as a client; checks for reachability, offline, retries and device revocation.
- Additional hosting acceptance: the same server package on a VPS, automatic installation through the SSH wizard, and connecting by address without changing the adapters. Manual installation is also checked.
- Check that changing the address to someone else's broker does not move the queue, and that the old and new nodes do not serve the same history at the same time.

The immediate work stays the same: a single local client and recovery. At the same time, we separate the broker from the client right away and do not hard-code a VPS requirement into the configuration or UI.
