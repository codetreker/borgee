#!/usr/bin/env node
import { run } from './cli.js';

void run(process.argv.slice(2))
  .then((code) => {
    process.exitCode = code;
  })
  .catch((err) => {
    console.error((err as Error).message);
    process.exitCode = 1;
  });
