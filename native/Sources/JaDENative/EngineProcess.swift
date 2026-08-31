import Foundation

final class EngineProcess {
    private(set) var process: Process?
    private(set) var baseURL: URL?
    private var generation = 0

    deinit {
        stop()
    }

    func start(root: URL, completion: @escaping (Result<URL, Error>) -> Void) {
        stop()
        let port = Int.random(in: 18_000...32_000)
        let address = "127.0.0.1:\(port)"
        let executable = engineExecutable()
        let process = Process()
        process.executableURL = executable
        process.arguments = ["-root", root.path, "-address", address]
        process.standardOutput = FileHandle.nullDevice
        process.standardError = FileHandle.nullDevice
        do {
            try process.run()
        } catch {
            completion(.failure(error))
            return
        }
        self.process = process
        let url = URL(string: "http://\(address)/")!
        baseURL = url
        waitUntilReady(url: url, attempts: 80, generation: generation, completion: completion)
    }

    func stop() {
        generation += 1
        if let process, process.isRunning {
            process.terminate()
        }
        process = nil
        baseURL = nil
    }

    private func waitUntilReady(
        url: URL,
        attempts: Int,
        generation: Int,
        completion: @escaping (Result<URL, Error>) -> Void
    ) {
        guard attempts > 0 else {
            completion(.failure(NSError(domain: "JaDE", code: 1, userInfo: [NSLocalizedDescriptionKey: "The Go engine did not become ready."])))
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
                    completion(.failure(NSError(domain: "JaDE", code: 2, userInfo: [NSLocalizedDescriptionKey: "The Go engine exited while starting."])))
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

    private func engineExecutable() -> URL {
        if let configured = ProcessInfo.processInfo.environment["JADE_ENGINE_PATH"], !configured.isEmpty {
            return URL(fileURLWithPath: configured)
        }
        if let bundled = Bundle.main.url(forResource: "jade-engine", withExtension: nil),
           FileManager.default.isExecutableFile(atPath: bundled.path) {
            return bundled
        }
        let development = URL(fileURLWithPath: FileManager.default.currentDirectoryPath)
            .appendingPathComponent(".tmp/jade-engine")
        return development
    }
}
