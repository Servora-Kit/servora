// Type declarations for custom Web Components used in this project.
import 'react'

declare module 'react' {
  namespace JSX {
    interface IntrinsicElements {
      'cap-widget': React.DetailedHTMLProps<
        React.HTMLAttributes<HTMLElement> & {
          'data-cap-api-endpoint'?: string
        },
        HTMLElement
      >
    }
  }
}
