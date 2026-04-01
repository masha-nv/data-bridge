const { app, BrowserWindow } = require("electron");
const path = require("path");
const { spawn } = require("child_process");

let mainWindow;
let goProcess;

function createWindow() {
  mainWindow = new BrowserWindow({
    width: 1200,
    height: 800,
    webPreferences: {
      nodeIntegration: false,
      contextIsolation: true,
    },
  });
  mainWindow.loadURL("http://localhost:4200"); // Angular dev server
  mainWindow.on("closed", function () {
    mainWindow = null;
    if (goProcess) goProcess.kill();
  });
}

app.on("ready", () => {
  // Start Go backend
  goProcess = spawn("go", ["run", "main.go"], {
    cwd: path.join(__dirname, "../backend"),
  });
  goProcess.stdout.on("data", (data) => {
    console.log(`Go: ${data}`);
  });
  goProcess.stderr.on("data", (data) => {
    console.error(`Go error: ${data}`);
  });
  createWindow();
});

app.on("window-all-closed", function () {
  if (process.platform !== "darwin") app.quit();
});

app.on("activate", function () {
  if (mainWindow === null) createWindow();
});
