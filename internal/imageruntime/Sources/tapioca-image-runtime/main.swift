import Foundation
import MLXQwenImageFlash
import MLXToolKit

struct Arguments {
    var model = ""
    var prompt = ""
    var output = ""
    var negativePrompt: String?
    var width = 1024
    var height = 1024
    var steps = 4
    var seed: UInt64 = 0

    init(_ values: [String]) throws {
        var index = 0
        while index < values.count {
            let name = values[index]
            guard index + 1 < values.count else {
                throw RuntimeError.usage("missing value for \(name)")
            }
            let value = values[index + 1]
            switch name {
            case "--model": model = value
            case "--prompt": prompt = value
            case "--output": output = value
            case "--negative-prompt": negativePrompt = value
            case "--width": width = try Self.integer(value, name)
            case "--height": height = try Self.integer(value, name)
            case "--steps": steps = try Self.integer(value, name)
            case "--seed":
                guard let parsed = UInt64(value) else { throw RuntimeError.usage("invalid seed") }
                seed = parsed
            default: throw RuntimeError.usage("unknown argument \(name)")
            }
            index += 2
        }
        guard !model.isEmpty, !prompt.isEmpty, !output.isEmpty else {
            throw RuntimeError.usage("--model, --prompt, and --output are required")
        }
    }

    private static func integer(_ value: String, _ name: String) throws -> Int {
        guard let parsed = Int(value) else { throw RuntimeError.usage("invalid \(name)") }
        return parsed
    }
}

enum RuntimeError: Error, LocalizedError {
    case usage(String)
    var errorDescription: String? {
        switch self { case .usage(let message): return message }
    }
}

@main
struct TapiocaImageRuntime {
    static func main() async throws {
        let args = try Arguments(Array(CommandLine.arguments.dropFirst()))
        let package = QwenImageFlashPackage(configuration: .init(
            snapshotPath: args.model,
            quant: .int8,
            defaultSteps: args.steps,
            defaultWidth: args.width,
            defaultHeight: args.height))
        try await package.load()
        let result = try await package.run(T2IRequest(
            prompt: args.prompt,
            negativePrompt: args.negativePrompt,
            width: args.width,
            height: args.height,
            steps: args.steps,
            guidanceScale: 1.0,
            seed: args.seed))
        guard let response = result as? T2IResponse else {
            throw RuntimeError.usage("image backend returned an unexpected response")
        }
        try response.image.data.write(to: URL(fileURLWithPath: args.output), options: .atomic)
        await package.unload()
    }
}
