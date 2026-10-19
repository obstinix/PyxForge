; Inno Setup script for PyxForge on Windows x64. tools/winpkg stages the files and runs ISCC:
;
;   go run ./tools/winpkg -version 3.0.0
;
; It defines AppVersion (3.0.0, 3.1.0-rc.1), NumericVersion (3.0.0.0), StageDir (the files to
; install), ExampleDir (examples/boot-sector as committed) and OutputDir.
;
; Install per user without administrator rights (all users is offered); nothing is downloaded
; or run during setup. Settings and projects live outside the install folder and survive
; upgrades; uninstalling removes settings only when the user says so.

#ifndef AppVersion
  #error Define AppVersion, NumericVersion, StageDir, ExampleDir and OutputDir (tools/winpkg does).
#endif

#define AppName "PyxForge"
#define AppExe "PyxForge.exe"
; Not "Documents\PyxForge": that is where people clone PyxForge itself.
#define ProjectsDir "{userdocs}\PyxForge Projects"

[Setup]
; Never change AppId: upgrades and uninstall find earlier installations by it.
AppId={{9737575F-C03A-42D7-AD71-EE54622C28BC}
AppName={#AppName}
AppVersion={#AppVersion}
AppVerName={#AppName} {#AppVersion}
AppPublisher=obstinix
AppPublisherURL=https://github.com/obstinix/PyxForge
AppSupportURL=https://github.com/obstinix/PyxForge/issues
AppUpdatesURL=https://github.com/obstinix/PyxForge/releases
AppCopyright=Copyright PyxForge contributors
VersionInfoVersion={#NumericVersion}
VersionInfoProductVersion={#NumericVersion}
VersionInfoProductTextVersion={#AppVersion}
VersionInfoDescription={#AppName} Setup
DefaultDirName={autopf}\{#AppName}
DefaultGroupName={#AppName}
DisableProgramGroupPage=yes
PrivilegesRequired=lowest
PrivilegesRequiredOverridesAllowed=dialog
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
MinVersion=10.0.17763
LicenseFile={#StageDir}\LICENSE.txt
SetupIconFile=pyxforge.ico
UninstallDisplayIcon={app}\{#AppExe}
UninstallDisplayName={#AppName}
OutputDir={#OutputDir}
OutputBaseFilename=PyxForge-Setup-{#AppVersion}
Compression=lzma2/max
SolidCompression=yes
WizardStyle=modern
; A running PyxForge is closed (and asked to save) before its files are replaced.
CloseApplications=yes
RestartApplications=no
ShowLanguageDialog=no

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"

[Tasks]
Name: "desktopicon"; Description: "{cm:CreateDesktopIcon}"; GroupDescription: "{cm:AdditionalIcons}"; Flags: unchecked

[InstallDelete]
; Neovim is replaced whole on upgrade, so files a newer Neovim dropped do not linger.
Type: filesandordirs; Name: "{app}\nvim"

[Dirs]
; Projects belong to the user: kept on uninstall.
Name: "{#ProjectsDir}"; Flags: uninsneveruninstall

[Files]
Source: "{#StageDir}\*"; DestDir: "{app}"; Flags: ignoreversion recursesubdirs createallsubdirs
; The example project is copied once and then belongs to the user: never replaced, never removed.
Source: "{#ExampleDir}\*"; DestDir: "{#ProjectsDir}\boot-sector"; Flags: onlyifdoesntexist uninsneveruninstall recursesubdirs createallsubdirs

[Icons]
Name: "{autoprograms}\{#AppName}"; Filename: "{app}\{#AppExe}"; Parameters: """{#ProjectsDir}"""; WorkingDir: "{#ProjectsDir}"; Comment: "Operating-system development workbench"
Name: "{autodesktop}\{#AppName}"; Filename: "{app}\{#AppExe}"; Parameters: """{#ProjectsDir}"""; WorkingDir: "{#ProjectsDir}"; Tasks: desktopicon

[Run]
Filename: "{app}\{#AppExe}"; Parameters: """{#ProjectsDir}"""; WorkingDir: "{#ProjectsDir}"; Description: "{cm:LaunchProgram,{#AppName}}"; Flags: nowait postinstall skipifsilent

[Code]
// Settings, caches and the editor's data live in the user's profile, not in {app}. They are
// removed only when the user asks; a silent uninstall keeps them.
procedure CurUninstallStepChanged(CurUninstallStep: TUninstallStep);
begin
  if (CurUninstallStep = usPostUninstall) and not UninstallSilent then
  begin
    if MsgBox('Also delete your PyxForge settings, caches and editor data?' + #13#10 + #13#10 +
              'Your projects (Documents\PyxForge Projects and any other folders you opened) are not touched.',
              mbConfirmation, MB_YESNO or MB_DEFBUTTON2) = IDYES then
    begin
      DelTree(ExpandConstant('{userappdata}\PyxForge'), True, True, True);
      DelTree(ExpandConstant('{userappdata}\fyne\io.github.obstinix.pyxforge'), True, True, True);
      DelTree(ExpandConstant('{localappdata}\PyxForge'), True, True, True);
      DelTree(ExpandConstant('{localappdata}\pyxforge-data'), True, True, True);
    end;
  end;
end;
