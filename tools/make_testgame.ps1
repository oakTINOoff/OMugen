<#
.SYNOPSIS
  Builds a zero-copy OMugen test folder from an existing MUGEN game.

.DESCRIPTION
  Big asset folders (chars, stages, sound, ...) become junctions to the game.
  data\ and font\ are real folders: the game's files are hard-linked in (no
  extra disk space) and Ikemen's own engine files are copied next to them.
  lib\ and external\ are junctions to the OMugen repo. Run again after each
  build to refresh OMugen.exe.

  Nothing is ever written into the original game folder by this script.
  Hard links share content with the original files, so never edit files in
  the test folder's data\ or font\ that came from the game.

.EXAMPLE
  powershell -ExecutionPolicy Bypass -File tools\make_testgame.ps1 -Game "D:\Jump Force Mugen" -Out "D:\OMugen\test\jf"
#>
param(
	[Parameter(Mandatory = $true)][string]$Game,
	[Parameter(Mandatory = $true)][string]$Out,
	[string]$Repo = ''
)
$ErrorActionPreference = 'Stop'
if (-not $Repo) { $Repo = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path) }

if (-not (Test-Path (Join-Path $Game 'data\mugen.cfg'))) { throw "$Game does not look like a MUGEN folder (no data\mugen.cfg)" }
$Game = (Resolve-Path $Game).Path
if (-not (Test-Path $Out)) { New-Item -ItemType Directory $Out | Out-Null }
$Out = (Resolve-Path $Out).Path
if ($Out.StartsWith($Game, [StringComparison]::OrdinalIgnoreCase)) { throw "Output must not be inside the game folder" }

# mklink takes names literally (New-Item -Target treats [brackets] as wildcards in PowerShell 5.1).
function Invoke-Mklink([string]$kind, [string]$link, [string]$target) {
	$out = cmd /c "mklink $kind `"$link`" `"$target`"" 2>&1
	if ($LASTEXITCODE -ne 0) { throw "mklink $kind failed for $link : $out" }
}

function Link-Dir([string]$link, [string]$target) {
	if (Test-Path -LiteralPath $link) { return }
	Invoke-Mklink '/J' $link $target
}

# Logs are written by the engine; never share them with the game.
function Skip-File($item) { return $item.Extension -eq '.log' }

# Mirror a game folder: subfolders as junctions, files as hard links.
function Mirror-Folder([string]$src, [string]$dst) {
	if (-not (Test-Path -LiteralPath $dst)) { New-Item -ItemType Directory $dst | Out-Null }
	foreach ($item in Get-ChildItem -LiteralPath $src -Force) {
		$to = Join-Path $dst $item.Name
		if (Test-Path -LiteralPath $to) { continue }
		if ($item.PSIsContainer) { Invoke-Mklink '/J' $to $item.FullName }
		elseif (-not (Skip-File $item)) { Invoke-Mklink '/H' $to $item.FullName }
	}
}

# Copy engine files, refusing to overwrite anything that came from the game.
function Copy-EngineFiles([string]$src, [string]$dst) {
	foreach ($item in Get-ChildItem -LiteralPath $src -Force) {
		$to = Join-Path $dst $item.Name
		if (Test-Path -LiteralPath $to) {
			$existing = Get-Item -LiteralPath $to -Force
			$fromGame = $existing.LinkType -in @('HardLink', 'Junction') -and -not ($existing.Target -like "$Repo*")
			if ($fromGame) { Write-Warning "Skipping $($item.Name): the game already has a file with that name"; continue }
		}
		Copy-Item -LiteralPath $item.FullName -Destination $to -Recurse -Force
	}
}

# Everything at the game root except data/font becomes a junction (folders) or hard link (files),
# skipping MUGEN's own executables and backups.
foreach ($item in Get-ChildItem -LiteralPath $Game -Force) {
	if ($item.Name -in @('data', 'font', 'save', 'lib', 'external')) { continue }
	if ($item.Name -match '\.(exe|Backup)$') { continue }
	$to = Join-Path $Out $item.Name
	if (Test-Path -LiteralPath $to) { continue }
	if ($item.PSIsContainer) { Link-Dir $to $item.FullName }
	elseif (-not (Skip-File $item)) { Invoke-Mklink '/H' $to $item.FullName }
}

Mirror-Folder (Join-Path $Game 'data') (Join-Path $Out 'data')
Copy-EngineFiles (Join-Path $Repo 'data') (Join-Path $Out 'data')

if (Test-Path (Join-Path $Game 'font')) { Mirror-Folder (Join-Path $Game 'font') (Join-Path $Out 'font') }
else { New-Item -ItemType Directory (Join-Path $Out 'font') -Force | Out-Null }
Copy-EngineFiles (Join-Path $Repo 'font') (Join-Path $Out 'font')

Link-Dir (Join-Path $Out 'external') (Join-Path $Repo 'external')
Link-Dir (Join-Path $Out 'lib') (Join-Path $Repo 'lib')

Copy-Item (Join-Path $Repo 'Ikemen_GO.exe') (Join-Path $Out 'OMugen.exe') -Force
$joyTest = Join-Path $Repo 'build\mugenjoytest.exe'
if (Test-Path $joyTest) { Copy-Item $joyTest (Join-Path $Out 'mugenjoytest.exe') -Force }
Write-Host "Test game ready: $Out\OMugen.exe"
