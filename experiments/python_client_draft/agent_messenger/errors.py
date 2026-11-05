"""Stable failures for the local client. None of these start a model."""


class Rejected(ValueError):
    """The request conflicts with the queue rules."""


class RecoveryBlocked(Rejected):
    """A resolution exists as a command, but this recovery path stays closed."""


class SupervisorBusy(RuntimeError):
    """Another supervisor already holds the canonical store lock."""


class SupervisorUnavailable(RuntimeError):
    """No supervisor is accepting commands. This does not start one."""


class AdapterError(RuntimeError):
    """A CLI event or launch command failed closed."""


class ConfigError(RuntimeError):
    """Configuration asked for a broker or mode this client will not start."""
