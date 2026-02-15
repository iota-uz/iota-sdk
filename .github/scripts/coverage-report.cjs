const fs = require('fs');
const { execSync } = require('child_process');

/**
 * Simplified Coverage Reporter - Analyzes and reports Go test coverage
 * Focuses on core functionality used in CI workflows
 */
class CoverageReporter {
  constructor(options = {}) {
    // Helper function for environment variable parsing
    const getEnvInt = (envVar, optionValue, defaultValue) =>
      process.env[envVar] ? parseInt(process.env[envVar]) : (optionValue || defaultValue);

    const getEnvValue = (envVar, optionValue, defaultValue) =>
      process.env[envVar] || optionValue || defaultValue;

    // Core configuration
    this.coverageFile = getEnvValue('COVERAGE_FILE', options.coverageFile, 'coverage.out');
    this.threshold = getEnvInt('COVERAGE_THRESHOLD', options.threshold, 70);
    this.outputFormat = getEnvValue('COVERAGE_OUTPUT', options.outputFormat, 'github');
    const maxBufferMBValue = process.env.COVERAGE_MAX_BUFFER_MB ?? options.maxBufferMB ?? 64;
    const parsedMaxBufferMB = Number.parseInt(maxBufferMBValue, 10);
    this.maxBufferMB = Number.isFinite(parsedMaxBufferMB) && parsedMaxBufferMB > 0 ? parsedMaxBufferMB : 64;
    this.maxBufferBytes = this.maxBufferMB * 1024 * 1024;

    // Display limits
    this.maxLowCoverageDisplay = getEnvInt('COVERAGE_MAX_LOW_COVERAGE_DISPLAY', null, 20);
    this.maxFilesDisplay = getEnvInt('COVERAGE_MAX_FILES_DISPLAY', null, 50);

    // Status thresholds
    this.thresholds = {
      excellent: getEnvInt('COVERAGE_THRESHOLD_EXCELLENT', null, 80),
      good: getEnvInt('COVERAGE_THRESHOLD_GOOD', null, 70),
      fair: getEnvInt('COVERAGE_THRESHOLD_FAIR', null, 60),
      poor: getEnvInt('COVERAGE_THRESHOLD_POOR', null, 40)
    };

    // Parse ignore patterns
    const defaultIgnore = 'cmd/,*_templ.go';
    const envIgnore = process.env.COVERAGE_IGNORE_PATTERNS;
    const rawPatterns = options.ignorePatterns || envIgnore || defaultIgnore;
    this.ignorePatterns = (Array.isArray(rawPatterns) ? rawPatterns : rawPatterns.split(','))
      .map(p => p.trim())
      .filter(p => p.length > 0);
  }

  checkCoverageFile() {
    if (!fs.existsSync(this.coverageFile)) {
      throw new Error(`Coverage file '${this.coverageFile}' not found`);
    }
  }

  /**
   * Simplified ignore check - combines file and package logic
   */
  shouldIgnore(filePath) {
    return this.ignorePatterns.some(pattern => {
      if (pattern.includes('*')) {
        // File pattern (e.g., *_templ.go)
        const regex = new RegExp(pattern.replace(/\*/g, '.*') + '$');
        return regex.test(filePath);
      } else {
        // Directory pattern (e.g., cmd/, viewmodels/)
        const cleanPattern = pattern.endsWith('/') ? pattern.slice(0, -1) : pattern;
        return filePath.includes(cleanPattern + '/') || filePath.startsWith(cleanPattern + '/');
      }
    });
  }

  /**
   * Parse coverage data from go tool cover output
   */
  parseCoverageData(lines) {
    const functions = [];
    const fileCoverage = {};

    lines
      .filter(line => line.includes('.go:') && !line.includes('total:'))
      .forEach(line => {
        const parts = line.split('\t').filter(p => p.length > 0);
        if (parts.length >= 2) {
          const pathWithLine = parts[0].trim();
          const functionName = parts.length >= 3 ? parts[parts.length - 2].trim() : '';
          const coverage = parts[parts.length - 1].trim();
          const coverageNum = parseFloat(coverage.replace('%', ''));

          // Extract file path
          const pathMatch = pathWithLine.match(/^(.+?)\/([^/]+\.go):(\d+):$/);
          if (pathMatch) {
            const filePath = pathMatch[1] + '/' + pathMatch[2];
            const fileName = pathMatch[2];
            const lineNumber = pathMatch[3];
            const shortPath = filePath.replace('github.com/iota-uz/iota-sdk/', '');

            // Skip if should be ignored
            if (this.shouldIgnore(shortPath)) return;

            // Add to functions
            functions.push({
              functionName,
              fileName,
              shortPath,
              lineNumber,
              coverage,
              coverageNum
            });

            // Add to file coverage stats
            if (!fileCoverage[shortPath]) {
              fileCoverage[shortPath] = { sum: 0, count: 0 };
            }
            fileCoverage[shortPath].sum += coverageNum;
            fileCoverage[shortPath].count++;
          }
        }
      });

    // Calculate file averages and sort
    const files = Object.entries(fileCoverage)
      .map(([filePath, stats]) => ({
        file: filePath,
        coverage: `${(stats.sum / stats.count).toFixed(1)}%`,
        coverageNum: stats.sum / stats.count
      }))
      .sort((a, b) => a.coverageNum - b.coverageNum);

    return { functions, files };
  }

  /**
   * Calculate total coverage from filtered functions
   */
  calculateTotalCoverage(functions) {
    if (functions.length === 0) return { percentage: 0, formatted: '0.0%' };

    const totalCoverage = functions.reduce((sum, func) => sum + func.coverageNum, 0);
    const avgCoverage = totalCoverage / functions.length;

    return {
      percentage: avgCoverage,
      formatted: `${avgCoverage.toFixed(1)}%`
    };
  }

  /**
   * Calculate coverage distribution
   */
  calculateDistribution(functions) {
    return {
      uncovered: functions.filter(f => f.coverageNum === 0).length,
      poor: functions.filter(f => f.coverageNum > 0 && f.coverageNum <= 25).length,
      fair: functions.filter(f => f.coverageNum > 25 && f.coverageNum <= 50).length,
      good: functions.filter(f => f.coverageNum > 50 && f.coverageNum <= 75).length,
      veryGood: functions.filter(f => f.coverageNum > 75 && f.coverageNum < 100).length,
      perfect: functions.filter(f => f.coverageNum === 100).length,
      get total() { return this.uncovered + this.poor + this.fair + this.good + this.veryGood + this.perfect; },
      get withTests() { return this.total - this.uncovered; }
    };
  }

  /**
   * Get coverage data
   */
  getCoverageData() {
    try {
      const output = execSync(`go tool cover -func="${this.coverageFile}"`, {
        encoding: 'utf8',
        maxBuffer: this.maxBufferBytes
      });
      const lines = output.trim().split('\n');

      // Parse coverage data
      const { functions, files } = this.parseCoverageData(lines);

      // Calculate totals
      const totalCoverage = this.calculateTotalCoverage(functions);
      const distribution = this.calculateDistribution(functions);

      // Get low coverage functions
      const lowCoverageFunctions = functions
        .filter(f => f.coverageNum < this.threshold)
        .sort((a, b) => a.coverageNum - b.coverageNum)
        .slice(0, this.maxLowCoverageDisplay);

      return {
        coverage: totalCoverage.formatted,
        coverageNum: totalCoverage.percentage,
        fileCount: functions.length,
        files: files.slice(0, this.maxFilesDisplay),
        lowCoverageFunctions,
        distribution,
        allFunctions: functions
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

  formatPercentage(value, total) {
    if (total === 0) return '0.0%';
    return `${((value / total) * 100).toFixed(1)}%`;
  }

  generateGitHubSummary(data) {
    const { status, color } = this.getStatus(data.coverageNum);
    const summaryFile = process.env.GITHUB_STEP_SUMMARY;

    if (!summaryFile) {
      console.warn('GITHUB_STEP_SUMMARY not set, outputting to console');
      return this.generateConsoleOutput(data);
    }

    const { distribution } = data;

    const summary = [
      '## 📊 Test Coverage Report',
      '',
      `![Coverage Badge](https://img.shields.io/badge/Coverage-${encodeURIComponent(data.coverage)}-${color}?style=for-the-badge&logo=go)`,
      '',
      '| Metric | Value | Threshold |',
      '|--------|-------|-----------|',
      `| **Total Coverage** | **${data.coverage}** | ${this.threshold}% |`,
      `| **Status** | ${status} | - |`,
      `| **Functions Tested** | ${distribution.withTests}/${distribution.total} (${this.formatPercentage(distribution.withTests, distribution.total)}) | - |`,
    ];

    if (this.ignorePatterns.length > 0) {
      summary.push(`| **Ignored Patterns** | ${this.ignorePatterns.join(', ')} | - |`);
    }

    summary.push(
      '',
      '### 📋 Coverage by File (sorted lowest to highest)',
      '',
      '| File | Coverage |',
      '|------|----------|',
      ...data.files.map(file => `| \`${file.file}\` | ${file.coverage} |`),
      ''
    );

    if (data.lowCoverageFunctions.length > 0) {
      summary.push(
        '<details>',
        `<summary>🔍 Functions with Low Coverage (< ${this.threshold}%)</summary>`,
        ''
      );

      // Group by file
      const grouped = {};
      data.lowCoverageFunctions.forEach(func => {
        if (!grouped[func.shortPath]) grouped[func.shortPath] = [];
        grouped[func.shortPath].push(func);
      });

      Object.entries(grouped).forEach(([filePath, functions]) => {
        summary.push(
          '',
          `**${filePath}**`,
          '| Function | Line | Coverage |',
          '|----------|------|----------|'
        );
        functions.forEach(func => {
          summary.push(`| \`${func.functionName}\` | ${func.lineNumber} | ${func.coverage} |`);
        });
      });

      summary.push('</details>', '');
    }

    // Status message
    if (data.coverageNum < this.threshold) {
      summary.push(`❌ Coverage ${data.coverage} is below the required threshold of ${this.threshold}%`);
    } else {
      summary.push('✅ Coverage meets the required threshold');
    }

    fs.appendFileSync(summaryFile, summary.join('\n') + '\n');
  }

  generateConsoleOutput(data) {
    const { status } = this.getStatus(data.coverageNum);
    const { distribution } = data;

    console.log('==================== Test Coverage Report ====================');
    console.log(`Total Coverage: ${data.coverage} (${status})`);
    console.log(`Functions Tested: ${distribution.withTests}/${distribution.total} (${this.formatPercentage(distribution.withTests, distribution.total)})`);
    console.log(`Threshold: ${this.threshold}%`);
    if (this.ignorePatterns.length > 0) {
      console.log(`Ignored Patterns: ${this.ignorePatterns.join(', ')}`);
    }
    console.log('');

    console.log(`📋 File Coverage (Lowest ${this.maxFilesDisplay} by coverage):`);
    console.log('----------------------------------------');
    data.files.forEach(file => {
      console.log(`${file.file.padEnd(50)} ${file.coverage}`);
    });

    if (data.lowCoverageFunctions.length > 0) {
      console.log('');
      console.log(`🔍 Functions with Low Coverage (< ${this.threshold}%):`);
      console.log('----------------------------------------');

      const grouped = {};
      data.lowCoverageFunctions.forEach(func => {
        if (!grouped[func.shortPath]) grouped[func.shortPath] = [];
        grouped[func.shortPath].push(func);
      });

      for (const [filePath, functions] of Object.entries(grouped)) {
        console.log(`\n${filePath}:`);
        functions.forEach(func => {
          console.log(`  L${func.lineNumber.padEnd(5)} ${func.functionName.padEnd(30)} ${func.coverage}`);
        });
      }
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

// Command line argument parsing (simplified)
function parseArguments(args) {
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
      case '--ignore':
        options.ignorePatterns = value ? value.split(',') : [];
        i++;
        break;
      case '-h':
      case '--help':
        console.log('Usage: node coverage-report.js [OPTIONS]');
        console.log('Options:');
        console.log('  -f, --file <file>     Coverage file (default: coverage.out)');
        console.log('  -t, --threshold <n>   Coverage threshold percentage (default: 70)');
        console.log('  -o, --output <format> Output format: github|console (default: github)');
        console.log('  --ignore <patterns>   Comma-separated patterns to ignore');
        console.log('  -h, --help           Show this help message');
        process.exit(0);
        break;
      default:
        if (flag.startsWith('-')) {
          console.error(`Unknown option: ${flag}`);
          process.exit(1);
        }
    }
  }

  return options;
}

// Main entry point
function main() {
  const args = process.argv.slice(2);
  const options = parseArguments(args);

  const reporter = new CoverageReporter(options);
  reporter.run();
}

if (require.main === module) {
  main();
}

module.exports = CoverageReporter;
