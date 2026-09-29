import { Link } from 'react-router-dom'
import { Icon } from './Brand.jsx'

export default function NotFound() {
  return (
    <div className="flex min-h-screen flex-col items-center justify-center gap-4 p-6 text-center">
      <Icon name="rca" size={48} className="text-rose-500" />
      <h1 className="text-2xl font-semibold text-slate-900 dark:text-slate-50">Page not found</h1>
      <p className="text-slate-500">The page you requested does not exist in this documentation.</p>
      <Link to="/" className="rounded-lg bg-teal-600 px-4 py-2 text-sm font-medium text-white hover:bg-teal-500">Back to the docs</Link>
    </div>
  )
}
