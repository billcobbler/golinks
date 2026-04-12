# Homebrew formula for the golinks CLI.
# This file is auto-updated by the release workflow — do not edit version/sha manually.
class Golinks < Formula
  desc "CLI tool for managing self-hosted golinks"
  homepage "https://github.com/billcobbler/golinks"
  version "GOLINKS_VERSION"

  on_macos do
    on_arm do
      url "https://github.com/billcobbler/golinks/releases/download/vGOLINKS_VERSION/golinks-darwin-arm64.tar.gz"
      sha256 "GOLINKS_SHA256_DARWIN_ARM64"
    end
    on_intel do
      url "https://github.com/billcobbler/golinks/releases/download/vGOLINKS_VERSION/golinks-darwin-amd64.tar.gz"
      sha256 "GOLINKS_SHA256_DARWIN_AMD64"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/billcobbler/golinks/releases/download/vGOLINKS_VERSION/golinks-linux-arm64.tar.gz"
      sha256 "GOLINKS_SHA256_LINUX_ARM64"
    end
    on_intel do
      url "https://github.com/billcobbler/golinks/releases/download/vGOLINKS_VERSION/golinks-linux-amd64.tar.gz"
      sha256 "GOLINKS_SHA256_LINUX_AMD64"
    end
  end

  def install
    # The binary in the archive has OS/arch suffix, rename it when installing
    binary_name = if OS.mac?
                    Hardware::CPU.arm? ? "golinks-darwin-arm64" : "golinks-darwin-amd64"
                  else
                    Hardware::CPU.arm? ? "golinks-linux-arm64" : "golinks-linux-amd64"
                  end
    bin.install binary_name => "golinks"
  end

  test do
    assert_match "golinks", shell_output("#{bin}/golinks --version 2>&1")
  end
end
