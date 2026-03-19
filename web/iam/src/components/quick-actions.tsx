import { Link } from '@tanstack/react-router'
import { Button } from '#/components/ui/button'
import { UserPlus, AppWindow } from 'lucide-react'

export function QuickActions() {
  return (
    <div className="space-y-2">
      <h3 className="text-sm font-medium text-muted-foreground">快捷操作</h3>
      <div className="flex flex-wrap gap-2">
        <Button variant="outline" size="sm" asChild>
          <Link to="/users">
            <UserPlus className="size-4" />
            管理用户
          </Link>
        </Button>
        <Button variant="outline" size="sm" asChild>
          <Link to="/applications">
            <AppWindow className="size-4" />
            注册应用
          </Link>
        </Button>
      </div>
    </div>
  )
}
