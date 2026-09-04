import Foundation

final class EngineProcess {
    private(set) var process: Process?
    private(set) var baseURL: URL?
    private var generation = 0
    private var stdoutHandle: FileHandle?
    private var stdoutBuffer = Data()

    deinit {
        stop()
    }

    func start(root: URL, completion: @escaping (Result<URL, Error>) -> Void) {
        stop()
        let generation = self.generation
        let process = Process()
        process.executableURL = engineExecutable()
        process.arguments = ["-root", root.path, "-address", "127.0.0.1:0"]
        let stdout = Pipe()
        process.standardOutput = stdout
        process.standardError = FileHandle.nullDevice
        do {
            try process.run()
        } catch {
            completion(.failure(error))
            return
        }
        self.process = process
        stdoutHandle = stdout.fileHandleForReading
        stdoutBuffer.removeAll()
        stdout.fileHandleForReading.readabilityHandler = { [weak self] handle in
            let data = handle.availableData
            DispatchQueue.main.async {
                guard let self, self.generation == generation else { return }
                self.consume(data, completion: completion)
            }
        }
        DispatchQueue.main.asyncAfter(deadline: .now() + 10) { [weak self] in
            guard let self, self.generation == generation, self.baseURL == nil else { return }
            self.stop()
            completion(.failure(Self.error(1, "The Go engine did not report its address.")))
        }
    }

    func stop() {
        generation += 1
        stdoutHandle?.readabilityHandler = nil
        stdoutHandle = nil
        stdoutBuffer.removeAll()
        if let process, process.isRunning {
            process.terminate()
        }
        process = nil
        baseURL = nil
    }

    private func consume(_ data: Data, completion: @escaping (Result<URL, Error>) -> Void) {
        guard baseURL == nil else { return }
        if data.isEmpty {
            stdoutHandle?.readabilityHandler = nil
            stop()
            completion(.failure(Self.error(2, "The Go engine exited while starting.")))
            return
        }
        stdoutBuffer.append(data)
        guard let text = String(data: stdoutBuffer, encoding: .utf8),
              let range = text.range(of: "http://"),
              let newline = text[range.lowerBound...].firstIndex(where: \.isNewline)
        else { return }
        let addressText = String(text[range.lowerBound..<newline])
        guard let url = URL(string: addressText.hasSuffix("/") ? addressText : addressText + "/") else {
            stop()
            completion(.failure(Self.error(3, "The Go engine reported an invalid address.")))
            return
        }
        baseURL = url
        waitUntilReady(url: url, attempts: 80, generation: generation, completion: completion)
    }

    private func waitUntilReady(
        url: URL,
        attempts: Int,
        generation: Int,
        completion: @escaping (Result<URL, Error>) -> Void
    ) {
        guard attempts > 0 else {
            stop()
            completion(.failure(Self.error(4, "The Go engine did not become ready.")))
            return
        }
        var request = URLRequest(url: url)
        request.timeoutInterval = 0.25
        URLSession.shared.dataTask(with: request) { [weak self] _, response, _ in
            DispatchQueue.main.async {
                guard let self, self.generation == generation else { return }
                if let response = response as? HTTPURLResponse, response.statusCode == 200 {
                    completion(.success(url))
                    return
                }
                guard self.process?.isRunning == true else {
                    self.stop()
                    completion(.failure(Self.error(2, "The Go engine exited while starting.")))
                    return
                }
                DispatchQueue.main.asyncAfter(deadline: .now() + 0.05) { [weak self] in
                    guard let self, self.generation == generation else { return }
                    self.waitUntilReady(
                        url: url,
                        attempts: attempts - 1,
                        generation: generation,
                        completion: completion
                    )
                }
            }
        }.resume()
    }

    private static func error(_ code: Int, _ message: String) -> NSError {
        NSError(domain: "JaDE", code: code, userInfo: [NSLocalizedDescriptionKey: message])
    }

    private func engineExecutable() -> URL {
        if let configured = ProcessInfo.processInfo.environment["JADE_ENGINE_PATH"], !configured.isEmpty {
            return URL(fileURLWithPath: configured)
        }
        if let bundled = Bundle.main.url(forResource: "jade-engine", withExtension: nil),
           FileManager.default.isExecutableFile(atPath: bundled.path) {
            return bundled
        }
        return URL(fileURLWithPath: FileManager.default.currentDirectoryPath)
            .appendingPathComponent(".tmp/jade-engine")
    }
}
