import { useState } from 'react'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '#/components/ui/alert-dialog'
import { Input } from '#/components/ui/input'

interface ConfirmDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  title: string
  description: string
  onConfirm: () => void
  confirmLabel?: string
  cancelLabel?: string
  destructive?: boolean
  /** When set, user must type this exact string to confirm */
  confirmInput?: string
  confirmInputPlaceholder?: string
}

export function ConfirmDialog({
  open,
  onOpenChange,
  title,
  description,
  onConfirm,
  confirmLabel = '确认',
  cancelLabel = '取消',
  destructive = false,
  confirmInput,
  confirmInputPlaceholder,
}: ConfirmDialogProps) {
  const [inputValue, setInputValue] = useState('')
  const requiresInput = typeof confirmInput === 'string'
  const canConfirm = !requiresInput || inputValue === confirmInput

  function handleConfirm() {
    onConfirm()
    onOpenChange(false)
    setInputValue('')
  }

  function handleOpenChange(next: boolean) {
    if (!next) setInputValue('')
    onOpenChange(next)
  }

  return (
    <AlertDialog open={open} onOpenChange={handleOpenChange}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>{title}</AlertDialogTitle>
          <AlertDialogDescription>{description}</AlertDialogDescription>
        </AlertDialogHeader>
        {requiresInput && (
          <div className="py-2">
            <Input
              value={inputValue}
              onChange={(e) => setInputValue(e.target.value)}
              placeholder={confirmInputPlaceholder ?? `请输入 "${confirmInput}" 以确认`}
            />
          </div>
        )}
        <AlertDialogFooter>
          <AlertDialogCancel>{cancelLabel}</AlertDialogCancel>
          <AlertDialogAction
            variant={destructive ? 'destructive' : 'default'}
            onClick={handleConfirm}
            disabled={!canConfirm}
          >
            {confirmLabel}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  )
}
