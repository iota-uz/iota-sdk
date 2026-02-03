import React from 'react'

declare global {
  namespace JSX {
    interface IntrinsicElements {
      'bi-chat-root': React.DetailedHTMLProps<React.HTMLAttributes<HTMLElement>, HTMLElement>
    }
  }
}

export {}
