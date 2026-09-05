import extensibility
from extensibility import InputTensor, OutputTensor, foreach
from max.gpu.host import DeviceContext
from std.utils.coord import Coord


@extensibility.register("mnist_relu")
struct Relu:
    @staticmethod
    def execute[target: StaticString](
        output: OutputTensor,
        x: InputTensor[dtype=output.dtype, rank=output.rank, ...],
        ctx: DeviceContext,
    ) raises:
        @parameter
        @always_inline
        def activate[width: Int](idx: Coord) -> SIMD[x.dtype, width]:
            var value = x.load[width](idx)
            return max(value, SIMD[x.dtype, width](0))

        foreach[activate, target=target](output, ctx)
