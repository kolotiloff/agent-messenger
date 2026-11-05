"""Queue and process behavior for the commands a future window will call.

These sentences are the command help and the status payload. They describe
what the running client actually does.
"""

PAUSE = (
    'Pause new work. The queue is kept. The active process is not signaled '
    'and is not marked completed. No new claims or launches. MCP for an '
    'attempt that is still running stays valid until that attempt is stopped '
    'or marked uncertain.'
)

UNPAUSE = (
    'Allow new claims again. Does not clear an uncertain or failed hold, '
    'does not signal the active process, and does not call a model by itself.'
)

STOP_CURRENT = (
    'Stop the current job. The attempt becomes uncertain and is not retried. '
    'Its MCP context can no longer read the message or write a reply. If a '
    'process group was recorded, a best-effort termination signal may be sent. '
    'That signal is not proof the process tree exited and does not show the '
    'work was undone. The session stays blocked for new launches.'
)

SHUTDOWN = (
    'Shut down this client and stop accepting commands. The queue stays on '
    'disk. The active process is not killed and is not marked completed. '
    'This command does not revoke MCP and does not adopt or reap the active '
    'process. The next start of the client finds any attempt it did not '
    'spawn, marks that attempt uncertain, revokes its MCP context, and does '
    'not launch the session again.'
)

LIMITATIONS = (
    'process_exit_proof_not_implemented',
    'missing_pid_is_not_proof',
    'empty_process_group_is_not_proof',
    'reply_text_is_not_acceptance',
    'no_timer_release',
    'no_automatic_retry',
    'broker_not_started',
    'gui_not_implemented',
    'autostart_not_implemented',
    'real_cli_not_run_by_default_tests',
)
