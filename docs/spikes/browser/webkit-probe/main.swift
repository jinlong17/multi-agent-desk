import AppKit
import Foundation
import WebKit

final class ProbeDelegate: NSObject, WKNavigationDelegate {
    private let webView: WKWebView
    private var attempts = 0

    init(webView: WKWebView) {
        self.webView = webView
    }

    func webView(_ webView: WKWebView, didFinish navigation: WKNavigation!) {
        poll()
    }

    func webView(
        _ webView: WKWebView,
        didFail navigation: WKNavigation!,
        withError error: Error
    ) {
        fail("navigation:\(type(of: error))")
    }

    func webView(
        _ webView: WKWebView,
        didFailProvisionalNavigation navigation: WKNavigation!,
        withError error: Error
    ) {
        fail("provisional-navigation:\(type(of: error))")
    }

    private func poll() {
        attempts += 1
        if attempts > 450 {
            fail("probe-timeout")
            return
        }
        webView.evaluateJavaScript(
            "document.documentElement.dataset.probeComplete === 'true'"
        ) { [weak self] value, error in
            guard let self else { return }
            if error != nil {
                self.fail("javascript-evaluation")
                return
            }
            if (value as? Bool) == true {
                self.readResult()
                return
            }
            DispatchQueue.main.asyncAfter(deadline: .now() + 0.1) {
                self.poll()
            }
        }
    }

    private func readResult() {
        webView.evaluateJavaScript("document.querySelector('#result').textContent") {
            value, error in
            guard error == nil, let result = value as? String else {
                self.fail("result-read")
                return
            }
            FileHandle.standardOutput.write(Data("\(result)\n".utf8))
            exit(EXIT_SUCCESS)
        }
    }

    private func fail(_ reason: String) {
        let output = "{\"schemaVersion\":1,\"harnessError\":\"\(reason)\"}\n"
        FileHandle.standardError.write(Data(output.utf8))
        exit(EXIT_FAILURE)
    }
}

guard CommandLine.arguments.count == 2,
      let url = URL(string: CommandLine.arguments[1]) else {
    FileHandle.standardError.write(Data("invalid probe URL\n".utf8))
    exit(EXIT_FAILURE)
}

let application = NSApplication.shared
application.setActivationPolicy(.prohibited)

let configuration = WKWebViewConfiguration()
configuration.websiteDataStore = .default()
let webView = WKWebView(
    frame: NSRect(x: 0, y: 0, width: 800, height: 600),
    configuration: configuration
)
let delegate = ProbeDelegate(webView: webView)
webView.navigationDelegate = delegate
webView.load(URLRequest(url: url))

application.run()
