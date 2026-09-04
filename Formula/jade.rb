class Jade < Formula
  desc "Local text and file workspace with nested project context"
  homepage "https://github.com/mcembalest/jade"
  head "https://github.com/mcembalest/jade.git", branch: "main"

  depends_on "go" => :build
  depends_on :macos

  def install
    system "go", "build", *std_go_args, "./cmd/jade"
  end

  test do
    (testpath/"jade.md").write("# Homebrew check\n")
    output = testpath/"server.log"
    pid = spawn bin/"jade", "--no-open", testpath.to_s, out: output.to_s, err: output.to_s
    begin
      url = nil
      50.times do
        url = output.read[/JaDE: (http:\/\/127\.0\.0\.1:\d+)/, 1] if output.exist?
        break if url
        sleep 0.1
      end
      refute_nil url, "JaDE did not report readiness"
      assert_match "Homebrew check", shell_output("curl --fail --silent #{url}")
    ensure
      Process.kill("TERM", pid)
      Process.wait(pid)
    end
  end
end
