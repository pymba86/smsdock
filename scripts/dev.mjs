import { spawn } from "node:child_process";

const tasks = [
  {
    name: "api",
    command: "go",
    args: ["run", "./cmd/api"],
    cwd: new URL("../packages/api/", import.meta.url),
    env: {
      SMSDOCK_HTTP_ADDR: ":8080",
      SMSDOCK_DB_PATH: "./data/smsdock.db",
      SMSDOCK_DEVICE_GLOBS: "/dev/serial/by-id/*,/dev/ttyUSB*",
    },
  },
  {
    name: "web",
    command: "pnpm",
    args: ["dev"],
    cwd: new URL("../packages/web/", import.meta.url),
  },
];

const children = [];
let shuttingDown = false;

function startTask(task) {
  const child = spawn(task.command, task.args, {
    cwd: task.cwd,
    env: {
      ...process.env,
      ...task.env,
    },
    stdio: "inherit",
  });

  child.on("exit", (code, signal) => {
    if (shuttingDown) {
      return;
    }

    const reason = signal ? `signal ${signal}` : `code ${code}`;
    console.error(`[smsdock] ${task.name} exited with ${reason}`);
    shutdown(code ?? 1);
  });

  children.push(child);
}

function shutdown(exitCode = 0) {
  shuttingDown = true;

  for (const child of children) {
    if (!child.killed) {
      child.kill("SIGTERM");
    }
  }

  setTimeout(() => process.exit(exitCode), 200).unref();
}

for (const task of tasks) {
  startTask(task);
}

for (const signal of ["SIGINT", "SIGTERM"]) {
  process.on(signal, () => shutdown(0));
}
