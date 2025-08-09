class ApitoCli < Formula
  desc "Apito CLI"
  homepage "https://apito.io"
  license "MIT"

  # Stable releases can be added by setting url and sha256 like below on tagging:
  # url "https://github.com/apito-io/cli/archive/refs/tags/vX.Y.Z.tar.gz"
  # sha256 "<replace-with-sha256>"

  head "https://github.com/apito-io/cli.git", branch: "main"

  depends_on "go" => :build

  def install
    ldflags = "-s -w -X main.version=#{version}"
    system "go", "build", *std_go_args(ldflags: ldflags)
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/apito --version")
  end
end

