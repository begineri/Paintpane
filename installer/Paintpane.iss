; Paths are relative to this script (installer/). Compile from anywhere: iscc installer\Paintpane.iss
#define VerFileNum FileOpen(AddBackslash(SourcePath) + "..\version.txt")
#define MyAppVersion Trim(StringChange(FileRead(VerFileNum),"v",""))

// Define platform based on command-line define from CI (or default)
#ifndef MyAppPlatform
  #define MyAppPlatform "amd64"
#endif

// Construct the dynamic base filename using the defined variables
#define MyBaseFilename "paintpane-setup-" + MyAppVersion + "-" + MyAppPlatform

[Setup]
AppMutex=Paintpane_SingleInstanceMutex
AppName=Paintpane
AppVersion={#MyAppVersion}
ArchitecturesAllowed=x64compatible and not arm64
DefaultDirName={commonpf64}\Paintpane
DefaultGroupName=Paintpane
OutputBaseFilename={#MyBaseFilename}
AppPublisher=begineri/Paintpane
SetupIconFile=Paintpane.ico
Compression=lzma2
SolidCompression=yes
OutputDir=..\bin
WizardStyle=modern
AppVerName=Paintpane - Museum Art Wallpapers v{#MyAppVersion}
DisableWelcomePage=no
WizardImageFile=Paintpane.bmp
UninstallDisplayIcon={app}\paintpane.exe
DisableProgramGroupPage=Yes

[Files]
Source: ..\bin\paintpane.exe; DestDir: {app}; Flags: ignoreversion
Source: Paintpane.ico; DestDir: {app}; Flags: ignoreversion

[Icons]
Name: {group}\Paintpane; Filename: {app}\paintpane.exe; IconFilename: {app}\Paintpane.ico

[Tasks]
Name: "startonboot"; Description: "Start with Windows"; GroupDescription: "Additional tasks"

[Registry]
Root: HKCU; Subkey: "SOFTWARE\Microsoft\Windows\CurrentVersion\Run"; ValueType: string; ValueName: "Paintpane"; ValueData: """{app}\paintpane.exe"" --autostart"; Flags: uninsdeletevalue; Tasks: startonboot

; setup.ini next to the exe tells the program that the installer has asked about starting with
; Windows (pkg/autostart.SetupMarker). Without it, the first run registers the program once, which
; is how the portable exe gets the switch on by default; with it, the answer above stands, ticked
; or not. The program only checks that the file exists; the key records the answer for support.
[INI]
Filename: "{app}\setup.ini"; Section: "Paintpane"; Key: "StartWithWindows"; String: "1"; Flags: uninsdeletesection; Tasks: startonboot
Filename: "{app}\setup.ini"; Section: "Paintpane"; Key: "StartWithWindows"; String: "0"; Flags: uninsdeletesection; Tasks: not startonboot

[UninstallDelete]
Type: files; Name: "{app}\setup.ini"
Type: files; Name: "{userappdata}\fyne\Paintpane\preferences.json"
Type: filesandordirs; Name: "{localappdata}\Temp\paintpane_downloads"
Type: filesandordirs; Name: "{localappdata}\Paintpane"
Type: filesandordirs; Name: "{commonstartup}\Paintpane"

[Run]
Filename: {app}\paintpane.exe; Description: Start Paintpane; Flags: postinstall nowait skipifsilent unchecked

[Code]
procedure InitializeWizard;
var
  RichViewer: TRichEditViewer;
begin
  RichViewer := TRichEditViewer.Create(WizardForm);
  RichViewer.Left := WizardForm.WelcomeLabel2.Left;
  RichViewer.Top := WizardForm.WelcomeLabel2.Top;
  RichViewer.Width := WizardForm.WelcomeLabel2.Width;
  RichViewer.Height := WizardForm.WelcomeLabel2.Height;
  RichViewer.Parent := WizardForm.WelcomeLabel2.Parent;
  RichViewer.BorderStyle := bsNone;
  RichViewer.TabStop := False;
  RichViewer.ReadOnly := True;
  WizardForm.WelcomeLabel2.Visible := False;
   
  RichViewer.RTFText:=
    '{\rtf2\ansi\deff0 {\fonttbl {\f0 Arial;}} \
    {\colortbl\red0\green0\blue0;} \
    \par \
    \fs18 Paintpane is an unofficial, modified build of Spice by Karl Kwong. \
    It is not affiliated with or supported by the Spice project.\par \
    \par \
    \fs18 You are installing version \b v{#MyAppVersion} \b0 \par \
    \par \
    \fs18 Report problems with this build on its own project page:\par \
    {\field{\*\fldinst{HYPERLINK "https://github.com/begineri/Paintpane"}}{\\fldrslt https://github.com/begineri/Paintpane}} \
    }';
end;
