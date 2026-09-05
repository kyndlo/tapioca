"""Standalone ONNX Runtime for Audio8 0.1B INT8 exports."""

__all__ = ["ArkTtsRuntime"]

# Tapioca: importing metadata or discovering tests must not initialize ONNX.
def __getattr__(name):
    if name == "ArkTtsRuntime":
        from .runtime import ArkTtsRuntime
        return ArkTtsRuntime
    raise AttributeError(name)
