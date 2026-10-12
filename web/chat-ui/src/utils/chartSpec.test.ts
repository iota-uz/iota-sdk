import { describe, expect, it } from 'vitest';
import { parseChartDataFromSpec, parseChartDataFromJsonString } from './chartSpec';

describe('chartSpec parsing', () => {
  it('parses apex options passed at top level', () => {
    const parsed = parseChartDataFromSpec({
      chart: { type: 'bar', height: 360 },
      title: { text: 'Top regions' },
      series: [{ name: 'Premium', data: [1040014964, 576000, 320000] }],
      xaxis: { categories: ['Tashkent', 'Region 2', 'Region 3'] },
      colors: ['#2563EB'],
    });

    expect(parsed).toMatchObject({
      chartType: 'bar',
      title: 'Top regions',
      series: [{ name: 'Premium', data: [1040014964, 576000, 320000] }],
      labels: ['Tashkent', 'Region 2', 'Region 3'],
      colors: ['#2563EB'],
      height: 360,
    });
    expect(parsed?.options).toBeDefined();
  });

  it('parses apex options wrapped under options key', () => {
    const parsed = parseChartDataFromJsonString(
      JSON.stringify({
        options: {
          chart: { type: 'line' },
          title: { text: 'Monthly trend' },
          series: [{ name: 'Sales', data: [10, 20, 30] }],
          xaxis: { categories: ['Jan', 'Feb', 'Mar'] },
        },
      })
    );

    expect(parsed).toMatchObject({
      chartType: 'line',
      title: 'Monthly trend',
      series: [{ name: 'Sales', data: [10, 20, 30] }],
      labels: ['Jan', 'Feb', 'Mar'],
      colors: undefined,
      height: undefined,
    });
    expect(parsed?.options).toBeDefined();
  });

  it('preserves logarithmic y-axis hint from apex options', () => {
    const parsed = parseChartDataFromSpec({
      chart: { type: 'bar' },
      title: { text: 'Spread' },
      series: [{ name: 'S', data: [1, 10000] }],
      yaxis: { logarithmic: true },
    });

    expect(parsed).toMatchObject({
      chartType: 'bar',
      logarithmic: true,
    });
  });
});
