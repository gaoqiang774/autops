param(
    [string]$Path = ".\test\123",
    [string]$FromDate = "",
    [string]$ToDate = "",
    [switch]$ByFile
)

$ErrorActionPreference = "Stop"

function New-QueryState {
    param([string]$FileName)

    [pscustomobject]@{
        FileName         = $FileName
        Time             = $null
        QueryTime        = $null
        RowsSent         = $null
        RowsExamined     = $null
        HasAllDocId      = $false
        HasAllDocPadId   = $false
        HasAllDocTitle   = $false
        HasAllDocSummary = $false
    }
}

function Add-MatchedQuery {
    param(
        [object]$State,
        [System.Collections.Generic.List[object]]$Rows
    )

    if (
        $State -and
        $State.Time -and
        $null -ne $State.RowsExamined -and
        $State.HasAllDocId -and
        $State.HasAllDocPadId -and
        $State.HasAllDocTitle -and
        $State.HasAllDocSummary
    ) {
        $timestamp = [datetimeoffset]::Parse($State.Time)
        $Rows.Add([pscustomobject]@{
            Date         = $timestamp.ToString("yyyy-MM-dd")
            File         = $State.FileName
            Time         = $State.Time
            RowsExamined = [long]$State.RowsExamined
            RowsSent     = [long]$State.RowsSent
            QueryTime    = [double]$State.QueryTime
        }) | Out-Null
    }
}

function Get-PctChange {
    param(
        [long]$Current,
        [Nullable[long]]$Previous
    )

    if ($null -eq $Previous -or $Previous -eq 0) {
        return $null
    }

    [math]::Round((($Current - $Previous) * 100.0 / $Previous), 1)
}

if (-not (Test-Path -LiteralPath $Path)) {
    throw "Path not found: $Path"
}

$files = Get-ChildItem -LiteralPath $Path -File | Sort-Object Name
$matchedRows = [System.Collections.Generic.List[object]]::new()

foreach ($file in $files) {
    $reader = [System.IO.StreamReader]::new($file.FullName)
    try {
        $state = New-QueryState $file.Name

        while (($line = $reader.ReadLine()) -ne $null) {
            if ($line.StartsWith("# Time:")) {
                Add-MatchedQuery $state $matchedRows
                $state = New-QueryState $file.Name
                $state.Time = $line.Substring(7).Trim()
                continue
            }

            if ($line.StartsWith("# Query_time:")) {
                if ($line -match "Query_time:\s*([0-9.]+)\s+Lock_time:\s*[0-9.]+\s+Rows_sent:\s*(\d+)\s+Rows_examined:\s*(\d+)") {
                    $state.QueryTime = $matches[1]
                    $state.RowsSent = $matches[2]
                    $state.RowsExamined = $matches[3]
                }
                continue
            }

            if ($line.Contains("allDoc.id")) {
                $state.HasAllDocId = $true
            }
            if ($line.Contains("allDoc.pad_id")) {
                $state.HasAllDocPadId = $true
            }
            if ($line.Contains("allDoc.title")) {
                $state.HasAllDocTitle = $true
            }
            if ($line.Contains("allDoc.summary")) {
                $state.HasAllDocSummary = $true
            }
        }

        Add-MatchedQuery $state $matchedRows
    }
    finally {
        $reader.Close()
    }
}

$filteredRows = $matchedRows
if ($FromDate) {
    $filteredRows = @($filteredRows | Where-Object { $_.Date -ge $FromDate })
}
if ($ToDate) {
    $filteredRows = @($filteredRows | Where-Object { $_.Date -le $ToDate })
}

if ($ByFile) {
    $groups = $filteredRows | Group-Object Date, File | Sort-Object Name
    $summary = foreach ($group in $groups) {
        $items = @($group.Group)
        $sum = ($items | Measure-Object RowsExamined -Sum).Sum
        [pscustomobject]@{
            Date                = $items[0].Date
            File                = $items[0].File
            Count               = $items.Count
            TotalRowsExamined   = [long]$sum
            TotalRowsExaminedYi = [math]::Round($sum / 100000000, 2)
            AvgRowsExamined     = [long]($sum / $items.Count)
        }
    }

    $summary | Format-Table -AutoSize
    return
}

$groups = $filteredRows | Group-Object Date | Sort-Object Name
$daily = foreach ($group in $groups) {
    $items = @($group.Group)
    $sum = ($items | Measure-Object RowsExamined -Sum).Sum
    [pscustomobject]@{
        Date                = $group.Name
        Count               = $items.Count
        TotalRowsExamined   = [long]$sum
        TotalRowsExaminedYi = [math]::Round($sum / 100000000, 2)
        AvgRowsExamined     = [long]($sum / $items.Count)
    }
}

$previous = $null
$outputRows = foreach ($row in $daily) {
    [pscustomobject]@{
        Date                = $row.Date
        Count               = $row.Count
        TotalRowsExamined   = $row.TotalRowsExamined
        TotalRowsExaminedYi = $row.TotalRowsExaminedYi
        DoD                 = if ($previous) { Get-PctChange $row.TotalRowsExamined $previous.TotalRowsExamined } else { $null }
        AvgRowsExamined     = $row.AvgRowsExamined
    }
    $previous = $row
}

$outputRows | Format-Table -AutoSize
