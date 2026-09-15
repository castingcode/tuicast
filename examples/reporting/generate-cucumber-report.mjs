import path from 'node:path'
import { generate } from 'multiple-cucumber-html-reporter'

const [jsonDir, reportPath] = process.argv.slice(2)
if (!jsonDir || !reportPath) {
  throw new Error('usage: generate-cucumber-report.mjs <cucumber-json-directory> <report-directory>')
}

await generate({
  jsonDir: path.resolve(jsonDir),
  reportPath: path.resolve(reportPath),
  reportName: 'TUICast Cucumber Examples',
  pageTitle: 'TUICast Cucumber Examples',
  openReportInBrowser: false,
  saveCollectedJSON: true,
  attachmentLayout: 'inline',
  hideMetadata: true,
  metadata: {
    browser: { name: 'TUICast', version: 'Go SDK' },
    device: 'Reference TUI',
    platform: { name: 'xterm-256color', version: '80x24' },
    executionPlatform: process.env.GITHUB_ACTIONS ? 'GitHub Actions' : 'local',
  },
})
