import katex from 'katex';
import { Converter } from 'showdown';
import React from 'react';

function formatOutput(str: string) {
  const latexRe = /\\\[([^\]]*)\\]/gm;
  const latexReRound = /\\\(([^)]*)\\\)/gm;
  let output = str;
  for (const re of [latexRe, latexReRound]) {
    for (const match of Array.from(output.matchAll(re))) {
      let html = katex.renderToString(match[1], {
        throwOnError: true,
      });
      if (str[(match.index || 0) - 1] === '\n') {
        html = `<br>${html}`;
      }
      output = output.replace(match[0], html);
    }
  }
  const converter = new Converter({
    tables: true,
  });
  return converter.makeHtml(output);
}

export type Props = {
    content: string;
} & React.HTMLAttributes<HTMLDivElement>;

export default function LlmOutput({ content, ...props }: Props) {
  return (
    <div {...props} dangerouslySetInnerHTML={{ __html: formatOutput(content) }} />
  );
}
