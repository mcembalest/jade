import AppKit
import WebKit

final class WorkspaceViewController: NSViewController {
    private let root: URL
    private let baseURL: URL
    private var webView: WKWebView!

    init(root: URL, baseURL: URL) {
        self.root = root.resolvingSymlinksInPath().standardizedFileURL
        self.baseURL = baseURL
        super.init(nibName: nil, bundle: nil)
    }

    required init?(coder: NSCoder) {
        fatalError("init(coder:) has not been implemented")
    }

    override func loadView() {
        let configuration = WKWebViewConfiguration()
        configuration.websiteDataStore = .default()
        webView = WKWebView(frame: .zero, configuration: configuration)
        view = webView
    }

    override func viewDidLoad() {
        super.viewDidLoad()
        webView.load(URLRequest(url: baseURL))
    }

    override func viewDidAppear() {
        super.viewDidAppear()
        view.window?.title = root.lastPathComponent + " — JaDE"
        view.window?.makeFirstResponder(webView)
    }

    func flushEditor(completion: @escaping (Bool) -> Void) {
        guard let webView else { completion(true); return }
        var finished = false
        let finish: (Bool) -> Void = { success in
            guard !finished else { return }
            finished = true
            if !success {
                let alert = NSAlert()
                alert.messageText = "Your edits have not been saved."
                alert.informativeText = "The workspace will stay open. Resolve the save error or download your edits before closing it."
                alert.addButton(withTitle: "Keep editing")
                alert.runModal()
            }
            completion(success)
        }
        webView.callAsyncJavaScript(
            "return window.__jadeFlush ? await window.__jadeFlush() : true",
            in: nil,
            in: .page
        ) { result in
            switch result {
            case .success(let value): finish(value as? Bool == true)
            case .failure: finish(false)
            }
        }
        DispatchQueue.main.asyncAfter(deadline: .now() + 12) { finish(false) }
    }
}
