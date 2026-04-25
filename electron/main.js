const { app, BrowserWindow } = require("electron");
const path = require("path");
const { spawn } = require("child_process");
const fs = require("fs");

let mainWindow;
let goProcess;

function wireBackendLogs(processHandle, label) {
  processHandle.stdout.on("data", (data) => {
    console.log(`[Go backend:${label}]: ${data}`);
  });
  processHandle.stderr.on("data", (data) => {
    console.error(`[Go backend error:${label}]: ${data}`);
  });
  processHandle.on("error", (err) => {
    console.error(`[Go backend failed to start:${label}]: ${err.message}`);
  });
}

function startBackend() {
  const backendPath = path.join(
    __dirname,
    process.platform === "win32" ? "backend.exe" : "backend",
  );

  if (fs.existsSync(backendPath)) {
    goProcess = spawn(backendPath, [], { cwd: __dirname });
    wireBackendLogs(goProcess, "binary");
    return;
  }

  const backendSourceDir = path.join(__dirname, "..", "backend");
  goProcess = spawn("go", ["run", "main.go"], {
    cwd: backendSourceDir,
    shell: true,
  });
  wireBackendLogs(goProcess, "go run");
  goProcess.on("exit", (code) => {
    if (code !== 0) {
      console.error(
        "[Go backend unavailable]: missing backend binary and failed to run source. Install Go (https://go.dev/dl/) then run npm run build-and-start from electron/.",
      );
    }
  });
}

function createWindow() {
  mainWindow = new BrowserWindow({
    width: 1200,
    height: 800,
    webPreferences: {
      nodeIntegration: false,
      contextIsolation: true,
    },
  });

  const indexPath = path.join(__dirname, "dist", "browser", "index.html");
  mainWindow.webContents.on("did-fail-load", (_event, code, desc, url) => {
    console.error(`[UI load failed] code=${code} desc=${desc} url=${url}`);
  });

  if (process.env.ELECTRON_DEBUG === "1") {
    mainWindow.webContents.openDevTools({ mode: "detach" });
  }

  // In production, serve Angular build from dist.
  // If the file is missing, render a clear local error page instead of a blank screen.
  if (!fs.existsSync(indexPath)) {
    console.error(`[UI missing] Expected file not found: ${indexPath}`);
    mainWindow.loadURL(
      "data:text/html," +
        encodeURIComponent(
          "<h2>Frontend bundle not found</h2><p>Run: npm run build-angular-prod && npm run copy-dist</p>",
        ),
    );
  } else {
    mainWindow.loadFile(indexPath);
  }

  mainWindow.on("closed", function () {
    mainWindow = null;
    if (goProcess) goProcess.kill();
  });
}

app.on("ready", () => {
  // Prefer prebuilt backend binary; fallback to `go run` during local development.
  startBackend();
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
