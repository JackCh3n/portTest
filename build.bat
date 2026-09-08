@echo off
REM Build script for Port Test Tool (Windows version)
REM Default: win-x86 + linux-x86 + linux-arm64
REM Usage:
REM   build.bat              REM build default: win-x86 + linux-x86 + linux-arm64
REM   build.bat win-x86      REM build win-x86 only
REM   build.bat linux-x86    REM build linux-x86 only
REM   build.bat linux-arm64  REM build linux-arm64 only (Kunpeng etc. ARM64)
REM   build.bat win          REM build all windows
REM   build.bat linux        REM build all linux
REM   build.bat all          REM build all platforms
REM   build.bat list         REM list supported platforms

setlocal EnableDelayedExpansion

set APP_NAME=port-test
set DIST_DIR=dist

if not exist "%DIST_DIR%" mkdir "%DIST_DIR%"

echo ====================================
echo   Port Test Tool - Build Script
echo ====================================
echo.

set TARGET=%1
if "%TARGET%"=="" set TARGET=default

if "%TARGET%"=="list" goto :list
if "%TARGET%"=="-l" goto :list
if "%TARGET%"=="--list" goto :list

if "%TARGET%"=="default" goto :default
if "%TARGET%"=="all" goto :all
if "%TARGET%"=="win" goto :win
if "%TARGET%"=="linux" goto :linux
if "%TARGET%"=="darwin" goto :darwin
if "%TARGET%"=="freebsd" goto :freebsd
if "%TARGET%"=="openbsd" goto :openbsd

REM Try as single label
call :build_one "%TARGET%"
goto :done

:default
echo Building default targets: win-x86, linux-x86, linux-arm64
echo.
call :build_one "win-x86"
call :build_one "linux-x86"
call :build_one "linux-arm64"
goto :done

:all
echo Building ALL platforms...
echo.
call :build_windows
call :build_linux
call :build_darwin
call :build_freebsd
call :build_openbsd
goto :done

:win
echo Building Windows group...
echo.
call :build_windows
goto :done

:linux
echo Building Linux group...
echo.
call :build_linux
goto :done

:darwin
echo Building macOS group...
echo.
call :build_darwin
goto :done

:freebsd
echo Building FreeBSD group...
echo.
call :build_freebsd
goto :done

:openbsd
echo Building OpenBSD group...
echo.
call :build_openbsd
goto :done

:done
echo.
echo Output directory: %DIST_DIR%\
echo.
dir /B "%DIST_DIR%\*" 2>nul
echo.
echo Done.
goto :eof

:build_windows
call :build_one "win-x86"
call :build_one "win-x64"
call :build_one "win-arm64"
goto :eof

:build_linux
call :build_one "linux-x86"
call :build_one "linux-x64"
call :build_one "linux-arm64"
call :build_one "linux-arm"
call :build_one "linux-mips64"
call :build_one "linux-mips64le"
call :build_one "linux-loong64"
goto :eof

:build_darwin
call :build_one "macos-x64"
call :build_one "macos-arm64"
goto :eof

:build_freebsd
call :build_one "freebsd-x64"
call :build_one "freebsd-arm64"
goto :eof

:build_openbsd
call :build_one "openbsd-x64"
goto :eof

:build_one
set LABEL=%~1

if "%LABEL%"=="win-x86" (
    set GOOS=windows
    set GOARCH=386
    set EXT=.exe
    set OUT=%DIST_DIR%\%APP_NAME%-win-x86.exe
)
if "%LABEL%"=="win-x64" (
    set GOOS=windows
    set GOARCH=amd64
    set EXT=.exe
    set OUT=%DIST_DIR%\%APP_NAME%-win-x64.exe
)
if "%LABEL%"=="win-arm64" (
    set GOOS=windows
    set GOARCH=arm64
    set EXT=.exe
    set OUT=%DIST_DIR%\%APP_NAME%-win-arm64.exe
)
if "%LABEL%"=="linux-x86" (
    set GOOS=linux
    set GOARCH=386
    set EXT=
    set OUT=%DIST_DIR%\%APP_NAME%-linux-x86
)
if "%LABEL%"=="linux-x64" (
    set GOOS=linux
    set GOARCH=amd64
    set EXT=
    set OUT=%DIST_DIR%\%APP_NAME%-linux-x64
)
if "%LABEL%"=="linux-arm64" (
    set GOOS=linux
    set GOARCH=arm64
    set EXT=
    set OUT=%DIST_DIR%\%APP_NAME%-linux-arm64
)
if "%LABEL%"=="linux-arm" (
    set GOOS=linux
    set GOARCH=arm
    set EXT=
    set OUT=%DIST_DIR%\%APP_NAME%-linux-arm
)
if "%LABEL%"=="linux-mips64" (
    set GOOS=linux
    set GOARCH=mips64
    set EXT=
    set OUT=%DIST_DIR%\%APP_NAME%-linux-mips64
)
if "%LABEL%"=="linux-mips64le" (
    set GOOS=linux
    set GOARCH=mips64le
    set EXT=
    set OUT=%DIST_DIR%\%APP_NAME%-linux-mips64le
)
if "%LABEL%"=="linux-loong64" (
    set GOOS=linux
    set GOARCH=loong64
    set EXT=
    set OUT=%DIST_DIR%\%APP_NAME%-linux-loong64
)
if "%LABEL%"=="macos-x64" (
    set GOOS=darwin
    set GOARCH=amd64
    set EXT=
    set OUT=%DIST_DIR%\%APP_NAME%-macos-x64
)
if "%LABEL%"=="macos-arm64" (
    set GOOS=darwin
    set GOARCH=arm64
    set EXT=
    set OUT=%DIST_DIR%\%APP_NAME%-macos-arm64
)
if "%LABEL%"=="freebsd-x64" (
    set GOOS=freebsd
    set GOARCH=amd64
    set EXT=
    set OUT=%DIST_DIR%\%APP_NAME%-freebsd-x64
)
if "%LABEL%"=="freebsd-arm64" (
    set GOOS=freebsd
    set GOARCH=arm64
    set EXT=
    set OUT=%DIST_DIR%\%APP_NAME%-freebsd-arm64
)
if "%LABEL%"=="openbsd-x64" (
    set GOOS=openbsd
    set GOARCH=amd64
    set EXT=
    set OUT=%DIST_DIR%\%APP_NAME%-openbsd-x64
)

echo Building %LABEL% (%GOOS%/%GOARCH%) -^> %OUT%
set "GOOS=%GOOS%"
set "GOARCH=%GOARCH%"
go build -ldflags="-s -w" -o "%OUT%" .
if errorlevel 1 (
    echo   Warning: build failed ^(Go may not support this target^)
    del "%OUT%" 2>nul
)
goto :eof

:list
echo Supported platforms:
echo.
echo   Labels ^(single target^):
echo     win-x86             (windows/386)
echo     win-x64             (windows/amd64)
echo     win-arm64           (windows/arm64)
echo     linux-x86           (linux/386)
echo     linux-x64           (linux/amd64)
echo     linux-arm64         (linux/arm64)
echo     linux-arm           (linux/arm)
echo     linux-mips64        (linux/mips64)
echo     linux-mips64le      (linux/mips64le)
echo     linux-loong64       (linux/loong64)
echo     macos-x64           (darwin/amd64)
echo     macos-arm64         (darwin/arm64)
echo     freebsd-x64         (freebsd/amd64)
echo     freebsd-arm64       (freebsd/arm64)
echo     openbsd-x64         (openbsd/amd64)
echo.
echo   Groups:
echo     win          all Windows platforms
echo     linux        all Linux platforms
echo     darwin       all macOS platforms
echo     freebsd      all FreeBSD platforms
echo     openbsd      all OpenBSD platforms
echo     all          all platforms above
echo.
echo Usage examples:
echo   build.bat                    default: win-x86 + linux-x86 + linux-arm64
echo   build.bat win-x86            single target
echo   build.bat win                all windows
echo   build.bat all                everything
goto :eof
