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

    func flushEditor(completion: @escaping () -> Void) {
        guard let webView, !webView.isLoading else {
            completion()
            return
        }
        var finished = false
        let finish = {
            if !finished {
                finished = true
                completion()
            }
        }
        webView.callAsyncJavaScript(
            "return window.__jadeFlush ? await window.__jadeFlush() : true",
            in: nil,
            in: .page
        ) { _ in finish() }
        DispatchQueue.main.asyncAfter(deadline: .now() + 3) { finish() }
    }

}
