Set-StrictMode -Version 2.0
$ErrorActionPreference = 'Stop'
$test_dir = Join-Path ([IO.Path]::GetTempPath()) ('wx-finder-test-' + [guid]::NewGuid().ToString('N'))
$batch = Join-Path $PSScriptRoot 'disable-wechat-liteapp-finder.bat'
$utf8 = New-Object System.Text.UTF8Encoding($false)
$old_search_root = $env:WX_FINDER_SEARCH_ROOT
$old_app_data = $env:APPDATA
$old_local_app_data = $env:LOCALAPPDATA

function New-TestConfig($users_root, $account, $version, $value) {
    $path = Join-Path $users_root "$account/xworker/liteapp/local/packages/ilinkres_uzhblidb/engineConfig/$version/dist/engineConfig.json"
    [IO.Directory]::CreateDirectory([IO.Path]::GetDirectoryName($path)) | Out-Null
    $text = '{"featureSwitch":{"handleFinder":' + $value + ',"keep":true},"other":{"large":123456789012345678901234567890,"deep":{"array":[{"keep":"value"}]}}}'
    [IO.File]::WriteAllText($path, $text, $utf8)
    return $path
}

function Invoke-TestScript($users_root, $expected_exit) {
    $ErrorActionPreference = 'Continue'
    if ([Environment]::OSVersion.Platform -eq [PlatformID]::Win32NT) {
        if ($users_root) { & $batch $users_root } else { & $batch }
    } else {
        $env:WX_FINDER_SEARCH_ROOT = $users_root
        & $powershell_executable -NoLogo -NoProfile -NonInteractive -File $extracted_script
    }
    if ($LASTEXITCODE -ne $expected_exit) {
        throw "Expected exit $expected_exit, got $LASTEXITCODE"
    }
}

try {
    [IO.Directory]::CreateDirectory($test_dir) | Out-Null
    $extracted_script = Join-Path $test_dir 'embedded.ps1'
    $source = [IO.File]::ReadAllText($batch)
    $marker = [regex]::Match($source, '(?m)^:__POWERSHELL__\r?$')
    if (-not $marker.Success) { throw 'Embedded script marker missing' }
    [IO.File]::WriteAllText($extracted_script, $source.Substring($marker.Index + $marker.Length), $utf8)
    $powershell_executable = (Get-Process -Id $PID).Path
    $users_root = Join-Path $test_dir (('users [space] & quote'' ! ') + [char]0x4e2d)
    $path_a = New-TestConfig $users_root 'user one' 10 true
    $path_b = New-TestConfig $users_root 'user two' 2 true
    $path_false = New-TestConfig $users_root 'user one' 8 false
    $ignored_path = New-TestConfig $users_root 'user two' 'not-a-version' true
    $original_a = [IO.File]::ReadAllText($path_a)
    $original_false = [IO.File]::ReadAllText($path_false)
    Invoke-TestScript $users_root 0
    if ([IO.File]::ReadAllText($path_a) -cne $original_a.Replace('"handleFinder":true', '"handleFinder":false')) {
        throw 'Unexpected changes to unrelated JSON text'
    }
    if ([IO.File]::ReadAllText($path_false) -cne $original_false) { throw 'Changed an already-false config' }
    if (-not (ConvertFrom-Json ([IO.File]::ReadAllText($ignored_path))).featureSwitch.handleFinder) { throw 'Changed a non-numeric version' }
    foreach ($path in @($path_a, $path_b)) {
        $backups = @(Get-ChildItem -LiteralPath ([IO.Path]::GetDirectoryName($path)) -Filter 'engineConfig.json.bak.*')
        if ($backups.Count -ne 1 -or [IO.File]::ReadAllText($backups[0].FullName) -cne $original_a) { throw 'Missing or incorrect backup' }
    }
    Invoke-TestScript $users_root 0
    if (@(Get-ChildItem -LiteralPath ([IO.Path]::GetDirectoryName($path_a)) -Filter '*.bak.*').Count -ne 1) { throw 'Repeated run created another backup' }

    $invalid_root = Join-Path $test_dir 'invalid'
    $invalid_path = New-TestConfig $invalid_root 'test-user' 1 '"true"'
    $invalid_text = [IO.File]::ReadAllText($invalid_path)
    Invoke-TestScript $invalid_root 1
    if ([IO.File]::ReadAllText($invalid_path) -cne $invalid_text) { throw 'Changed an invalid config' }
    [IO.File]::WriteAllText($invalid_path, '{invalid json', $utf8)
    Invoke-TestScript $invalid_root 1
    if ([IO.File]::ReadAllText($invalid_path) -cne '{invalid json') { throw 'Changed malformed JSON' }
    $ambiguous_text = '{"featureSwitch":{"handleFinder":true},"other":{"handleFinder":true}}'
    [IO.File]::WriteAllText($invalid_path, $ambiguous_text, $utf8)
    Invoke-TestScript $invalid_root 1
    if ([IO.File]::ReadAllText($invalid_path) -cne $ambiguous_text) { throw 'Changed an ambiguous config' }
    [IO.File]::WriteAllText($invalid_path, '{}', $utf8)
    Invoke-TestScript $invalid_root 1
    if ([IO.File]::ReadAllText($invalid_path) -cne '{}') { throw 'Changed a config without the setting' }
    $leftovers = @(Get-ChildItem -LiteralPath ([IO.Path]::GetDirectoryName($invalid_path)) | Where-Object { $_.Name -match '^engineConfig\.json\.(bak|tmp)\.' })
    if ($leftovers.Count -ne 0) { throw 'Left files after invalid input' }
    Invoke-TestScript (Join-Path $test_dir 'missing') 1
    $empty_root = Join-Path $test_dir 'empty'
    [IO.Directory]::CreateDirectory($empty_root) | Out-Null
    Invoke-TestScript $empty_root 1

    $env:APPDATA = Join-Path $test_dir 'Roaming'
    $env:LOCALAPPDATA = Join-Path $test_dir 'Local'
    $default_path = New-TestConfig (Join-Path $env:APPDATA 'Tencent/xwechat/radium/users') 'auto-user' 15 true
    $local_path = New-TestConfig (Join-Path $env:LOCALAPPDATA 'Tencent/WeChat/radium/users') 'local-user' 12 true
    Invoke-TestScript '' 0
    foreach ($path in @($default_path, $local_path)) {
        if ((ConvertFrom-Json ([IO.File]::ReadAllText($path))).featureSwitch.handleFinder) { throw 'Default discovery failed' }
    }
    Write-Host 'PASS: discovery, special paths, backups, JSON preservation, repeated runs and invalid configs'
} finally {
    $env:WX_FINDER_SEARCH_ROOT = $old_search_root
    $env:APPDATA = $old_app_data
    $env:LOCALAPPDATA = $old_local_app_data
    if ([IO.Directory]::Exists($test_dir)) { [IO.Directory]::Delete($test_dir, $true) }
}
