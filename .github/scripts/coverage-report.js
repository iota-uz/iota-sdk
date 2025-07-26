const fs = require('fs');
const { execSync } = require('child_process');

class CoverageReporter {
  constructor(options = {}) {
    this.coverageFile = options.coverageFile || process.env.COVERAGE_FILE || 'coverage.out';
    // Check environment variable first, then options, then default
    this.threshold = process.env.COVERAGE_THRESHOLD ? parseInt(process.env.COVERAGE_THRESHOLD) : (options.threshold || 70);
    this.outputFormat = options.outputFormat || process.env.COVERAGE_OUTPUT || 'github';
    
    // Status thresholds from environment variables
    this.thresholds = {
      excellent: process.env.COVERAGE_THRESHOLD_EXCELLENT ? parseInt(process.env.COVERAGE_THRESHOLD_EXCELLENT) : 80,
      good: process.env.COVERAGE_THRESHOLD_GOOD ? parseInt(process.env.COVERAGE_THRESHOLD_GOOD) : 70,
      fair: process.env.COVERAGE_THRESHOLD_FAIR ? parseInt(process.env.COVERAGE_THRESHOLD_FAIR) : 60,
      poor: process.env.COVERAGE_THRESHOLD_POOR ? parseInt(process.env.COVERAGE_THRESHOLD_POOR) : 40
    };
    
    // Display limits from environment variables
    this.maxLowCoverageDisplay = process.env.COVERAGE_MAX_LOW_COVERAGE_DISPLAY ? parseInt(process.env.COVERAGE_MAX_LOW_COVERAGE_DISPLAY) : 20;
    this.maxPackagesDisplay = process.env.COVERAGE_MAX_PACKAGES_DISPLAY ? parseInt(process.env.COVERAGE_MAX_PACKAGES_DISPLAY) : 15;
  }

  checkCoverageFile() {
    if (!fs.existsSync(this.coverageFile)) {
      throw new Error(`Coverage file '${this.coverageFile}' not found`);
    }
  }

  getCoverageData() {
    try {
      const output = execSync(`go tool cover -func="${this.coverageFile}"`, { encoding: 'utf8' });
      const lines = output.trim().split('\n');

      // Parse total coverage
      const totalLine = lines.find(line => line.includes('total:'));
      if (!totalLine) {
        throw new Error('Could not find total coverage in output');
      }

      const coverageMatch = totalLine.match(/(\d+(?:\.\d+)?)%/);
      if (!coverageMatch) {
        throw new Error('Could not parse coverage percentage');
      }

      const coverage = parseFloat(coverageMatch[1]);
      const coverageStr = `${coverage.toFixed(1)}%`;

      // Parse package coverage
      const packages = lines
        .filter(line => !line.includes('.go:') && !line.includes('total:') && line.trim())
        .map(line => {
          const parts = line.trim().split(/\s+/);
          if (parts.length >= 3) {
            const pkg = parts[0].replace('github.com/iota-uz/iota-sdk/', '');
            const cov = parts[2];
            return { package: pkg, coverage: cov, coverageNum: parseFloat(cov.replace('%', '')) };
          }
          return null;
        })
        .filter(Boolean)
        .sort((a, b) => b.coverageNum - a.coverageNum);

      // Parse function coverage
      const functions = lines
        .filter(line => line.includes('.go:'))
        .map(line => {
          const parts = line.trim().split(/\s+/);
          if (parts.length >= 3) {
            const func = parts[0];
            const cov = parts[2];
            const covNum = parseFloat(cov.replace('%', ''));
            return { function: func, coverage: cov, coverageNum: covNum };
          }
          return null;
        })
        .filter(Boolean);

      const lowCoverageFunctions = functions
        .filter(f => f.coverageNum > 0 && f.coverageNum < this.threshold)
        .sort((a, b) => a.coverageNum - b.coverageNum);

      return {
        coverage: coverageStr,
        coverageNum: coverage,
        fileCount: functions.length,
        packages: packages.slice(0, this.maxPackagesDisplay),
        lowCoverageFunctions: lowCoverageFunctions.slice(0, this.maxLowCoverageDisplay)
      };
    } catch (error) {
      throw new Error(`Failed to get coverage data: ${error.message}`);
    }
  }

  getStatus(coverage) {
    if (coverage >= this.thresholds.excellent) return { status: '🟢 Excellent', color: 'brightgreen' };
    if (coverage >= this.thresholds.good) return { status: '🟢 Good', color: 'green' };
    if (coverage >= this.thresholds.fair) return { status: '🟡 Fair', color: 'yellow' };
    if (coverage >= this.thresholds.poor) return { status: '🟠 Poor', color: 'orange' };
    return { status: '🔴 Critical', color: 'red' };
  }

  generateGitHubSummary(data) {
    const { status, color } = this.getStatus(data.coverageNum);
    const summaryFile = process.env.GITHUB_STEP_SUMMARY;

    if (!summaryFile) {
      console.warn('GITHUB_STEP_SUMMARY not set, outputting to console');
      return this.generateConsoleOutput(data);
    }

    const summary = [
      '## 📊 Test Coverage Report',
      '',
      `![Coverage Badge](https://img.shields.io/badge/Coverage-${encodeURIComponent(data.coverage)}-${color}?style=for-the-badge&logo=go)`,
      '',
      '| Metric | Value | Threshold |',
      '|--------|-------|-----------|',
      `| **Total Coverage** | **${data.coverage}** | ${this.threshold}% |`,
      `| **Status** | ${status} | - |`,
      `| **Files Tested** | ${data.fileCount} | - |`,
      '',
      '### 📋 Coverage by Package',
      '',
      '| Package | Coverage |',
      '|---------|----------|',
      ...data.packages.map(pkg => `| \`${pkg.package}\` | ${pkg.coverage} |`),
      ''
    ];

    if (data.lowCoverageFunctions.length > 0) {
      summary.push(
        '<details>',
        `<summary>🔍 Functions with Low Coverage (< ${this.threshold}%)</summary>`,
        '',
        '```',
        ...data.lowCoverageFunctions.map(f => `${f.function} ${f.coverage}`),
        '```',
        '</details>',
        ''
      );
    }

    // Coverage status
    if (data.coverageNum < this.threshold) {
      summary.push(`❌ Coverage ${data.coverage} is below the required threshold of ${this.threshold}%`);
    } else {
      summary.push('✅ Coverage meets the required threshold');
    }

    fs.appendFileSync(summaryFile, summary.join('\n') + '\n');
  }

  generateConsoleOutput(data) {
    const { status } = this.getStatus(data.coverageNum);

    console.log('==================== Test Coverage Report ====================');
    console.log(`Total Coverage: ${data.coverage} (${status})`);
    console.log(`Files Tested: ${data.fileCount}`);
    console.log(`Threshold: ${this.threshold}%`);
    console.log('');
    console.log('Package Coverage (Top 10):');
    console.log('----------------------------------------');

    data.packages.slice(0, 10).forEach(pkg => {
      console.log(`${pkg.package.padEnd(50)} ${pkg.coverage}`);
    });

    if (data.lowCoverageFunctions.length > 0) {
      console.log('');
      console.log(`Functions with Low Coverage (< ${this.threshold}%):`);
      console.log('----------------------------------------');
      data.lowCoverageFunctions.slice(0, 10).forEach(f => {
        console.log(`${f.function} ${f.coverage}`);
      });
    }

    console.log('');
    console.log('=============================================================');
  }

  setGitHubOutputs(data) {
    const { status, color } = this.getStatus(data.coverageNum);
    const outputFile = process.env.GITHUB_OUTPUT;

    if (outputFile) {
      const outputs = [
        `coverage=${data.coverage}`,
        `coverage_num=${data.coverageNum}`,
        `status=${status}`,
        `badge_color=${color}`
      ];
      fs.appendFileSync(outputFile, outputs.join('\n') + '\n');
    }
  }

  checkThreshold(coverage) {
    if (coverage < this.threshold) {
      if (process.env.GITHUB_ACTIONS === 'true') {
        console.log(`::warning title=Low Coverage::Test coverage (${coverage}%) is below the required threshold (${this.threshold}%)`);
      }
      throw new Error(`Coverage ${coverage}% is below the required threshold of ${this.threshold}%`);
    }
    console.log(`Coverage threshold check passed: ${coverage}%`);
  }

  run() {
    try {
      this.checkCoverageFile();
      const data = this.getCoverageData();

      console.log(`Total Coverage: ${data.coverage}`);

      if (this.outputFormat === 'github') {
        this.generateGitHubSummary(data);
      } else if (this.outputFormat === 'console') {
        this.generateConsoleOutput(data);
      } else {
        throw new Error(`Unknown output format: ${this.outputFormat}`);
      }

      this.setGitHubOutputs(data);
      this.checkThreshold(data.coverageNum);

    } catch (error) {
      console.error(`Error: ${error.message}`);
      process.exit(1);
    }
  }
}

// CLI interface
function main() {
  const args = process.argv.slice(2);
  const options = {};

  for (let i = 0; i < args.length; i++) {
    const flag = args[i];
    const value = args[i + 1];

    switch (flag) {
      case '-f':
      case '--file':
        options.coverageFile = value;
        i++;
        break;
      case '-t':
      case '--threshold':
        options.threshold = parseInt(value);
        i++;
        break;
      case '-o':
      case '--output':
        options.outputFormat = value;
        i++;
        break;
      case '-h':
      case '--help':
        console.log('Usage: node coverage-report.js [OPTIONS]');
        console.log('');
        console.log('Options:');
        console.log('  -f, --file       Coverage file (default: coverage.out)');
        console.log('  -t, --threshold  Coverage threshold percentage (default: 70)');
        console.log('  -o, --output     Output format: github|console (default: github)');
        console.log('  -h, --help       Show this help message');
        console.log('');
        console.log('Environment variables:');
        console.log('  COVERAGE_FILE                    Coverage file path (default: coverage.out)');
        console.log('  COVERAGE_THRESHOLD               Coverage threshold percentage (default: 70)');
        console.log('  COVERAGE_OUTPUT                  Output format: github|console (default: github)');
        console.log('  COVERAGE_THRESHOLD_EXCELLENT     Excellent status threshold (default: 80)');
        console.log('  COVERAGE_THRESHOLD_GOOD          Good status threshold (default: 70)');
        console.log('  COVERAGE_THRESHOLD_FAIR          Fair status threshold (default: 60)');
        console.log('  COVERAGE_THRESHOLD_POOR          Poor status threshold (default: 40)');
        console.log('  COVERAGE_MAX_LOW_COVERAGE_DISPLAY Max low-coverage functions shown (default: 20)');
        console.log('  COVERAGE_MAX_PACKAGES_DISPLAY    Max packages shown in summary (default: 15)');
        console.log('');
        console.log('Priority: CLI options > Environment variables > Defaults');
        process.exit(0);
      default:
        console.error(`Unknown option: ${flag}`);
        process.exit(1);
    }
  }

  const reporter = new CoverageReporter(options);
  reporter.run();
}

if (require.main === module) {
  main();
}

module.exports = CoverageReporter;
