# Downloads the latest shellrecap release for this machine into the current
# folder and starts it:
#
#   irm https://raw.githubusercontent.com/ksauraj/shellrecap/master/setup.ps1 | iex
#
# Works in Windows PowerShell 5.1 and PowerShell 7. It runs in its own
# scope, so it leaves no variables behind in the shell it's piped into.
& {
    $ErrorActionPreference = 'Stop'
    $repo = 'ksauraj/shellrecap'

    # PROCESSOR_ARCHITEW6432 is the real one when 32-bit PowerShell runs
    # on 64-bit Windows
    $arch = $env:PROCESSOR_ARCHITEW6432
    if (-not $arch) { $arch = $env:PROCESSOR_ARCHITECTURE }
    switch ($arch) {
        'AMD64' { $arch = 'amd64' }
        'ARM64' { $arch = 'arm64' }
        default { throw "shellrecap has no build for $arch Windows. See https://github.com/$repo/releases" }
    }

    $url = "https://github.com/$repo/releases/latest/download/shellrecap-windows-$arch.exe"
    $folder = (Get-Location -PSProvider FileSystem).ProviderPath
    $exe = Join-Path $folder 'shellrecap.exe'
    $download = "$exe.download"

    Write-Host "Downloading shellrecap for windows/$arch..."
    # Windows PowerShell 5.1 needs to be told to use TLS 1.2, which GitHub
    # requires, and is slow while it draws download progress
    [Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12
    $progress = $ProgressPreference
    $ProgressPreference = 'SilentlyContinue'
    try {
        Invoke-WebRequest -Uri $url -OutFile $download -UseBasicParsing
    }
    catch {
        Remove-Item -Force -ErrorAction SilentlyContinue $download
        throw "Couldn't download $url : $($_.Exception.Message)"
    }
    finally {
        $ProgressPreference = $progress
    }
    Move-Item -Force $download $exe
    Write-Host "Downloaded $exe"

    & $exe
}
