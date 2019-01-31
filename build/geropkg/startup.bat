@echo off 
set CURRENT=%cd%
set LIB_PATH=%CURRENT%\czero\lib
set CONFIG_PATH==%CURRENT%\geroConfig.toml
set path=%LIB_PATH%
set DATADIR=""
set KEYSTORE=""
set d=%1
if "%d%" neq "" (
   set DATADIR=--datadir  %d%
)
set k=%2
if "%k%" neq "" (
   set KEYSTORE=--keystore  %k%
)
start /b bin\gero.exe --config %CONFIG_PATH% %DATADIR% %KEYSTORE%

