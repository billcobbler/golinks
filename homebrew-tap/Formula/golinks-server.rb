# Homebrew formula for the golinks server.
# This file is auto-updated by the release workflow — do not edit version/sha manually.
class GolinksServer < Formula
  desc "Self-hosted golinks redirect server with web dashboard"
  homepage "https://github.com/billcobbler/golinks"
  version "GOLINKS_VERSION"

  on_macos do
    on_arm do
      url "https://github.com/billcobbler/golinks/releases/download/vGOLINKS_VERSION/golinks-server-darwin-arm64.tar.gz"
      sha256 "GOLINKS_SHA256_SERVER_DARWIN_ARM64"
    end
    on_intel do
      url "https://github.com/billcobbler/golinks/releases/download/vGOLINKS_VERSION/golinks-server-darwin-amd64.tar.gz"
      sha256 "GOLINKS_SHA256_SERVER_DARWIN_AMD64"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/billcobbler/golinks/releases/download/vGOLINKS_VERSION/golinks-server-linux-arm64.tar.gz"
      sha256 "GOLINKS_SHA256_SERVER_LINUX_ARM64"
    end
    on_intel do
      url "https://github.com/billcobbler/golinks/releases/download/vGOLINKS_VERSION/golinks-server-linux-amd64.tar.gz"
      sha256 "GOLINKS_SHA256_SERVER_LINUX_AMD64"
    end
  end

  def install
    # The binary in the archive has OS/arch suffix, rename it when installing
    binary_name = if OS.mac?
                    Hardware::CPU.arm? ? "golinks-server-darwin-arm64" : "golinks-server-darwin-amd64"
                  else
                    Hardware::CPU.arm? ? "golinks-server-linux-arm64" : "golinks-server-linux-amd64"
                  end
    bin.install binary_name => "golinks-server"
    (var/"golinks").mkpath
  end

  service do
    run [opt_bin/"golinks-server"]
    environment_variables GOLINKS_DB: var/"golinks/golinks.db",
                          GOLINKS_PORT: "8080"
    keep_alive true
    log_path var/"log/golinks-server.log"
    error_log_path var/"log/golinks-server-error.log"
  end

  test do
    port = free_port
    env = { "GOLINKS_PORT" => port.to_s, "GOLINKS_DB" => "#{testpath}/test.db" }
    pid = fork { exec(env, "#{bin}/golinks-server") }
    sleep 1
    assert_match "ok", shell_output("curl -sf http://localhost:#{port}/-/api/health")
  ensure
    Process.kill("TERM", pid)
  end
end
