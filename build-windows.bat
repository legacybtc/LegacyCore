@echo off
powershell.exe -ExecutionPolicy Bypass -File "%~dp0scripts\build-windows.ps1" %*
