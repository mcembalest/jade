import CPTY
import Darwin
import Foundation
import GhosttyTerminal

final class PTYSession: @unchecked Sendable {
    private let ioQueue = DispatchQueue(label: "com.cerebellica.jade.pty", qos: .userInteractive)
    private let queueKey = DispatchSpecificKey<Void>()
    private var masterFD: Int32 = -1
    private var childPID: pid_t = -1
    private var readSource: DispatchSourceRead?
    private var processSource: DispatchSourceProcess?
    private let startedAt = ContinuousClock.now

    private(set) var terminalSession: InMemoryTerminalSession!

    init(directory: String) throws {
        ioQueue.setSpecific(key: queueKey, value: ())
        terminalSession = InMemoryTerminalSession(
            write: { [weak self] data in self?.write(data) },
            resize: { [weak self] viewport in self?.resize(viewport) },
            suppressesPixelOnlyResizes: true
        )
        try start(directory: directory)
    }

    deinit {
        stop()
    }

    func stop() {
        if DispatchQueue.getSpecific(key: queueKey) != nil {
            shutdown()
        } else {
            ioQueue.sync { shutdown() }
        }
    }

    private func shutdown() {
        readSource?.cancel()
        readSource = nil
        processSource?.cancel()
        processSource = nil
        if masterFD >= 0 {
            Darwin.close(masterFD)
            masterFD = -1
        }
        let pid = childPID
        childPID = -1
        guard pid > 0 else { return }
        if Darwin.kill(-pid, SIGHUP) != 0 {
            Darwin.kill(pid, SIGHUP)
        }
        DispatchQueue.global(qos: .utility).async {
            var status: Int32 = 0
            while waitpid(pid, &status, 0) < 0, errno == EINTR {}
        }
    }

    private func start(directory: String) throws {
        let shell = ProcessInfo.processInfo.environment["SHELL"] ?? "/bin/zsh"
        var fd: Int32 = -1
        let pid = directory.withCString { directoryPointer in
            shell.withCString { shellPointer in
                jade_spawn_pty(directoryPointer, shellPointer, &fd)
            }
        }
        guard pid > 0, fd >= 0 else {
            throw POSIXError(POSIXErrorCode(rawValue: errno) ?? .EIO)
        }
        masterFD = fd
        childPID = pid

        let flags = fcntl(fd, F_GETFL)
        _ = fcntl(fd, F_SETFL, flags | O_NONBLOCK)

        let reader = DispatchSource.makeReadSource(fileDescriptor: fd, queue: ioQueue)
        reader.setEventHandler { [weak self] in self?.drainOutput() }
        reader.setCancelHandler {}
        readSource = reader
        reader.resume()

        let process = DispatchSource.makeProcessSource(identifier: pid, eventMask: .exit, queue: ioQueue)
        process.setEventHandler { [weak self] in self?.processExited() }
        processSource = process
        process.resume()
    }

    private func drainOutput() {
        guard masterFD >= 0 else { return }
        var buffer = [UInt8](repeating: 0, count: 64 * 1024)
        while true {
            let count = Darwin.read(masterFD, &buffer, buffer.count)
            if count > 0 {
                terminalSession.receive(Data(buffer.prefix(count)))
                continue
            }
            if count < 0, errno == EAGAIN || errno == EWOULDBLOCK {
                return
            }
            return
        }
    }

    private func write(_ data: Data) {
        ioQueue.async { [weak self] in
            guard let self, self.masterFD >= 0 else { return }
            data.withUnsafeBytes { bytes in
                guard let base = bytes.baseAddress else { return }
                var offset = 0
                while offset < bytes.count {
                    let count = Darwin.write(self.masterFD, base.advanced(by: offset), bytes.count - offset)
                    if count > 0 {
                        offset += count
                    } else if errno != EINTR {
                        return
                    }
                }
            }
        }
    }

    private func resize(_ viewport: InMemoryTerminalViewport) {
        ioQueue.async { [weak self] in
            guard let self, self.masterFD >= 0 else { return }
            _ = jade_resize_pty(self.masterFD, viewport.columns, viewport.rows)
        }
    }

    private func processExited() {
        guard childPID > 0 else { return }
        var status: Int32 = 0
        let pid = childPID
        while waitpid(pid, &status, 0) < 0, errno == EINTR {}
        childPID = -1
        readSource?.cancel()
        readSource = nil
        processSource?.cancel()
        processSource = nil
        if masterFD >= 0 {
            Darwin.close(masterFD)
            masterFD = -1
        }
        let code = (status & 0x7f) == 0 ? UInt32((status >> 8) & 0xff) : 1
        let elapsed = startedAt.duration(to: .now).components
        let milliseconds = elapsed.seconds * 1_000 + elapsed.attoseconds / 1_000_000_000_000_000
        terminalSession.finish(exitCode: code, runtimeMilliseconds: UInt64(max(0, milliseconds)))
    }
}
