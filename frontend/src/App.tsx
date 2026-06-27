import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import Dashboard from './pages/Dashboard'
import MigrationDetail from './pages/MigrationDetail'
import NewPlan from './pages/NewPlan'
import DriftScanPage from './pages/DriftScanPage'

export default function App() {
  return (
    <BrowserRouter>
      <div className="app">
        <nav className="nav">
          <a href="/" className="nav-brand">MSE Dashboard</a>
          <div className="nav-links">
            <a href="/">Migrations</a>
            <a href="/plans/new">New Plan</a>
            <a href="/drift-scan">Drift Scan</a>
          </div>
        </nav>
        <main className="main">
          <Routes>
            <Route path="/" element={<Dashboard />} />
            <Route path="/migrations/:id" element={<MigrationDetail />} />
            <Route path="/plans/new" element={<NewPlan />} />
            <Route path="/drift-scan" element={<DriftScanPage />} />
            <Route path="*" element={<Navigate to="/" replace />} />
          </Routes>
        </main>
      </div>
    </BrowserRouter>
  )
}
