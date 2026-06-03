const { app, BrowserWindow, Menu } = require('electron');
const { spawn } = require('child_process');
const net = require('net');
const path = require('path');

let mainWindow;
let serverProcess;

const SERVER_PORT = 8080;
const SERVER_URL = `http://localhost:${SERVER_PORT}`;

function waitForServer(port, retries, callback) {
  const socket = new net.Socket();
  let done = false;

  socket.setTimeout(500);
  socket.on('connect', () => {
    done = true;
    socket.destroy();
    callback(true);
  });
  const fail = () => {
    if (done) return;
    done = true;
    socket.destroy();
    if (retries > 0) {
      setTimeout(() => waitForServer(port, retries - 1, callback), 300);
    } else {
      callback(false);
    }
  };
  socket.on('timeout', fail);
  socket.on('error', fail);
  socket.connect(port, '127.0.0.1');
}

function spawnServer() {
  // Look for the binary next to this file (packaged) or two levels up (dev)
  const candidates = [
    path.join(__dirname, '..', 'auto-router'),
    path.join(__dirname, 'auto-router'),
  ];
  const binary = candidates.find(p => {
    try { require('fs').accessSync(p); return true; } catch { return false; }
  });

  if (!binary) return; // already running externally, skip

  const configPath = path.join(__dirname, '..', 'configs', 'config.yaml');
  serverProcess = spawn(binary, ['--config', configPath], {
    stdio: 'ignore',
    detached: false,
  });
}

function createWindow() {
  mainWindow = new BrowserWindow({
    width: 1280,
    height: 850,
    minWidth: 1024,
    minHeight: 700,
    frame: false,
    titleBarStyle: 'hidden',
    titleBarOverlay: {
      color: '#000000',
      symbolColor: '#666666',
      height: 32
    },
    backgroundColor: '#000000',
    title: 'Router Desktop Console',
    webPreferences: {
      nodeIntegration: true,
      contextIsolation: false,
      webSecurity: false
    }
  });

  Menu.setApplicationMenu(null);

  waitForServer(SERVER_PORT, 30, (ready) => {
    if (ready) {
      mainWindow.loadURL(SERVER_URL);
    } else {
      mainWindow.loadURL(`data:text/html,<h2>Server failed to start on port ${SERVER_PORT}</h2>`);
    }
  });

  mainWindow.on('closed', () => { mainWindow = null; });
}

app.whenReady().then(() => {
  // Try to spawn the Go binary if it isn't already running
  waitForServer(SERVER_PORT, 1, (alreadyUp) => {
    if (!alreadyUp) spawnServer();
    // Give it a moment then open the window (waitForServer inside createWindow retries)
    setTimeout(createWindow, alreadyUp ? 0 : 500);
  });

  app.on('activate', () => {
    if (BrowserWindow.getAllWindows().length === 0) createWindow();
  });
});

app.on('window-all-closed', () => {
  if (serverProcess) serverProcess.kill();
  if (process.platform !== 'darwin') app.quit();
});
