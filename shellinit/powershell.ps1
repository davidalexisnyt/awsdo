# awsdo history: Up/Down step through awsdo history while the line starts with
# "awsdo ". Ctrl+x,h does the same as Up. Elsewhere the keys keep their
# normal behavior.
$global:AwsdoEntries = @()
$global:AwsdoPos = 0
$global:AwsdoShown = $null
$global:AwsdoDraft = ''

function global:Step-AwsdoHistory([int]$Direction) {
    $line = $null
    $cursor = $null
    [Microsoft.PowerShell.PSConsoleReadLine]::GetBufferState([ref]$line, [ref]$cursor)

    if ($null -eq $global:AwsdoShown -or $line -ne $global:AwsdoShown) {
        if (-not $line.StartsWith('awsdo ')) { return $false }
        $global:AwsdoEntries = @(awsdo history --raw 2>$null)
        $global:AwsdoPos = $global:AwsdoEntries.Count
        $global:AwsdoDraft = $line
    }

    $new = [Math]::Max(0, $global:AwsdoPos + $Direction)
    if ($new -ge $global:AwsdoEntries.Count) {
        $new = $global:AwsdoEntries.Count
        $text = $global:AwsdoDraft
    } else {
        $text = "awsdo $($global:AwsdoEntries[$new])"
    }

    $global:AwsdoPos = $new
    $global:AwsdoShown = $text
    [Microsoft.PowerShell.PSConsoleReadLine]::Replace(0, $line.Length, $text)
    [Microsoft.PowerShell.PSConsoleReadLine]::SetCursorPosition($text.Length)
    return $true
}

$awsdoUp = {
    if (-not (Step-AwsdoHistory -1)) { [Microsoft.PowerShell.PSConsoleReadLine]::PreviousHistory() }
}
$awsdoDown = {
    if (-not (Step-AwsdoHistory 1)) { [Microsoft.PowerShell.PSConsoleReadLine]::NextHistory() }
}

Set-PSReadLineKeyHandler -Key UpArrow -ScriptBlock $awsdoUp
Set-PSReadLineKeyHandler -Key DownArrow -ScriptBlock $awsdoDown
Set-PSReadLineKeyHandler -Chord 'Ctrl+x,h' -ScriptBlock $awsdoUp
