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
  // In production, serve Angular build from dist
  mainWindow.loadFile(path.join(__dirname, "dist", "browser", "index.html"));
  mainWindow.on("closed", function () {
    mainWindow = null;
    if (goProcess) goProcess.kill();
  });
}

app.on("ready", () => {
  // Start Go backend binary (must be built and placed in electron/ as 'backend')
  const backendPath = path.join(
    __dirname,
    process.platform === "win32" ? "backend.exe" : "backend",
  );
  goProcess = spawn(backendPath, [], { cwd: __dirname });
  goProcess.stdout.on("data", (data) => {
    console.log(`[Go backend]: ${data}`);
  });
  goProcess.stderr.on("data", (data) => {
    console.error(`[Go backend error]: ${data}`);
  });
  createWindow();
});
app.on("quit", () => {
  if (goProcess) goProcess.kill();
});

app.on("window-all-closed", function () {
  if (process.platform !== "darwin") app.quit();
});

app.on("activate", function () {
  if (mainWindow === null) createWindow();
});
