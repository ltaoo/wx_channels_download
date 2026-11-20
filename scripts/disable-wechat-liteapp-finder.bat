@echo off
setlocal DisableDelayedExpansion
if not "%~2"=="" (
    echo Usage: "%~nx0" [radium\users directory]
    exit /b 1
)
set "WX_FINDER_BATCH=%~f0"
set "WX_FINDER_SEARCH_ROOT=%~1"
powershell.exe -NoLogo -NoProfile -NonInteractive -ExecutionPolicy Bypass -Command "$source = [IO.File]::ReadAllText($env:WX_FINDER_BATCH); $marker = [regex]::Match($source, '(?m)^:__POWERSHELL__\r?$'); if (-not $marker.Success) { throw 'Missing embedded PowerShell script' }; & ([ScriptBlock]::Create($source.Substring($marker.Index + $marker.Length)))"
exit /b %ERRORLEVEL%

:__POWERSHELL__
Set-StrictMode -Version 2.0
$ErrorActionPreference = 'Stop'
$temporary_path = $null

try {
    $search_roots = @()
    if ($env:WX_FINDER_SEARCH_ROOT) {
        if (-not (Test-Path -LiteralPath $env:WX_FINDER_SEARCH_ROOT -PathType Container)) {
            throw "Users directory not found: $env:WX_FINDER_SEARCH_ROOT"
        }
        $search_roots = @((Get-Item -LiteralPath $env:WX_FINDER_SEARCH_ROOT).FullName)
    } else {
        foreach ($app_data in @($env:APPDATA, $env:LOCALAPPDATA)) {
            if (-not $app_data) { continue }
            foreach ($product in @('xwechat', 'WeChat')) {
                $search_roots += Join-Path $app_data "Tencent\$product\radium\users"
            }
        }
    }

    $config_files = @()
    foreach ($search_root in ($search_roots | Sort-Object -Unique)) {
        if (-not (Test-Path -LiteralPath $search_root -PathType Container)) { continue }
        foreach ($account_dir in (Get-ChildItem -LiteralPath $search_root -Directory)) {
            if ($account_dir.Attributes -band [IO.FileAttributes]::ReparsePoint) { continue }
            $engine_root = Join-Path $account_dir.FullName 'xworker/liteapp/local/packages/ilinkres_uzhblidb/engineConfig'
            if (-not (Test-Path -LiteralPath $engine_root -PathType Container)) { continue }
            foreach ($version_dir in (Get-ChildItem -LiteralPath $engine_root -Directory)) {
                if ($version_dir.Name -notmatch '^[0-9]+$') { continue }
                if ($version_dir.Attributes -band [IO.FileAttributes]::ReparsePoint) { continue }
                $config_path = Join-Path $version_dir.FullName 'dist/engineConfig.json'
                if (Test-Path -LiteralPath $config_path -PathType Leaf) {
                    $config_files += Get-Item -LiteralPath $config_path
                }
            }
        }
    }

    $found_count = 0
    $changed_count = 0
    $utf8 = New-Object System.Text.UTF8Encoding($false)
    foreach ($config_file in ($config_files | Sort-Object FullName -Unique)) {
        if ($config_file.Directory.Parent.Name -notmatch '^[0-9]+$') { continue }
        if ($config_file.Attributes -band [IO.FileAttributes]::ReparsePoint) {
            Write-Host "Skipped symbolic link: $($config_file.FullName)"
            continue
        }
        $found_count++
        $config_path = $config_file.FullName
        $original_bytes = [IO.File]::ReadAllBytes($config_path)
        $original_text = [IO.File]::ReadAllText($config_path)
        $config_object = ConvertFrom-Json -InputObject $original_text
        if ($config_object.featureSwitch.handleFinder -isnot [bool]) {
            throw "featureSwitch.handleFinder must be a boolean: $config_path"
        }
        if (-not $config_object.featureSwitch.handleFinder) {
            Write-Host "Already false: $config_path"
            continue
        }

        # Preserve all other JSON text, including large integers and nested data.
        $flag_pattern = [regex]'("handleFinder"\s*:\s*)true\b'
        if ($flag_pattern.Matches($original_text).Count -ne 1) {
            throw "Cannot uniquely locate handleFinder=true: $config_path"
        }
        $modified_text = $flag_pattern.Replace($original_text, '${1}false', 1)
        $verified_object = ConvertFrom-Json -InputObject $modified_text
        if ($verified_object.featureSwitch.handleFinder -isnot [bool] -or $verified_object.featureSwitch.handleFinder) {
            throw "Modified JSON validation failed: $config_path"
        }

        $suffix = [guid]::NewGuid().ToString('N')
        $backup_path = "$config_path.bak.$suffix"
        $temporary_path = "$config_path.tmp.$suffix"
        [IO.File]::WriteAllText($temporary_path, $modified_text, $utf8)
        if ([Convert]::ToBase64String([IO.File]::ReadAllBytes($config_path)) -cne [Convert]::ToBase64String($original_bytes)) {
            throw "Config changed in another process; refusing to overwrite: $config_path"
        }
        [IO.File]::Replace($temporary_path, $config_path, $backup_path)
        $temporary_path = $null
        $changed_count++
        Write-Host "Updated to false: $config_path"
        Write-Host "Backup: $backup_path"
    }

    if ($found_count -eq 0) {
        throw 'No engine config found. Open Channels in WeChat once, or pass the radium\users directory as the first argument.'
    }
    Write-Host "Checked $found_count config(s), updated $changed_count. Fully exit and reopen WeChat."
} catch {
    [Console]::Error.WriteLine($_.Exception.Message)
    exit 1
} finally {
    if ($temporary_path -and [IO.File]::Exists($temporary_path)) {
        [IO.File]::Delete($temporary_path)
    }
}
exit 0
