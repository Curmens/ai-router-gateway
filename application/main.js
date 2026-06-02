const { app, BrowserWindow, Menu } = require('electron');
const path = require('path');
const net = require('net');

let mainWindow;

// Quick helper to test if local dev port 5173 is active before launching URL
function checkDevServer(port, host, timeoutMs, callback) {
  const socket = new net.Socket();
  let completed = false;

  socket.setTimeout(timeoutMs);

  socket.on('connect', () => {
    completed = true;
    socket.destroy();
    callback(true);
  });

  const handleError = () => {
    if (!completed) {
      completed = true;
      socket.destroy();
      callback(false);
    }
  };

  socket.on('timeout', handleError);
  socket.on('error', handleError);
  socket.connect(port, host);
}

function createWindow() {
  mainWindow = new BrowserWindow({
    width: 1280,
    height: 850,
    minWidth: 1024,
    minHeight: 700,
    frame: false, // Make window borderless / frameless like Discord
    titleBarStyle: 'hidden', // Native frameless window integration
    titleBarOverlay: {
      color: '#1e1f22',
      symbolColor: '#949ba4',
      height: 22
    },
    backgroundColor: '#1e1f22', // Match dark frame background during load
    title: 'Router Desktop Console',
    webPreferences: {
      nodeIntegration: true,
      contextIsolation: false,
      webSecurity: false
    }
  });

  // Remove default chromium standard menu bar for complete app immersion
  Menu.setApplicationMenu(null);

  // Probe port 5173 on localhost before choosing loader route
  checkDevServer(5173, '127.0.0.1', 600, (isRunning) => {
    if (isRunning) {
      mainWindow.loadURL('http://localhost:5173').catch(() => {
        mainWindow.loadFile(path.join(__dirname, 'dist', 'index.html'));
      });
      // Detach developer tools inside a separate window for clean styling in dev
      mainWindow.webContents.openDevTools({ mode: 'detach' });
    } else {
      mainWindow.loadFile(path.join(__dirname, 'dist', 'index.html'));
    }
  });

  mainWindow.on('closed', () => {
    mainWindow = null;
  });
}

app.whenReady().then(() => {
  createWindow();

  app.on('activate', () => {
    if (BrowserWindow.getAllWindows().length === 0) createWindow();
  });
});

app.on('window-all-closed', () => {
  if (process.platform !== 'darwin') app.quit();
});
