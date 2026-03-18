import { cn } from '#/lib/utils'

type Tone = 'green' | 'yellow' | 'red' | 'zinc'

// All tones use CSS design tokens defined in styles.css.
// success / warning / destructive map directly to theme semantic colors.
const toneStyles: Record<Tone, string> = {
  green:  'border-success/30 bg-success/10 text-success dark:border-success/20 dark:bg-success/15',
  yellow: 'border-warning/40 bg-warning/10 text-warning-foreground dark:border-warning/30 dark:bg-warning/15 dark:text-warning',
  red:    'border-destructive/30 bg-destructive/10 text-destructive dark:border-destructive/20 dark:bg-destructive/15',
  zinc:   'border-border bg-muted text-muted-foreground',
}

interface ToneBadgeProps {
  tone: Tone
  children: React.ReactNode
  className?: string
}

export function ToneBadge({ tone, children, className }: ToneBadgeProps) {
  return (
    <span
      className={cn(
        'inline-flex items-center rounded-md border px-2 py-0.5 text-xs font-medium',
        toneStyles[tone],
        className,
      )}
    >
      {children}
    </span>
  )
}

export function statusTone(status: string): Tone {
  switch (status) {
    case 'active':
    case 'accepted':
      return 'green'
    case 'pending':
    case 'invited':
      return 'yellow'
    case 'rejected':
    case 'deleted':
      return 'red'
    default:
      return 'zinc'
  }
}
