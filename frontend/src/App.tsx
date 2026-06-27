import { BrowserRouter, Routes, Route, Navigate, NavLink } from 'react-router-dom'
import ErrorBoundary from './components/ErrorBoundary'
import { ToastProvider } from './components/Toast'
import Dashboard from './pages/Dashboard'
import MigrationDetail from './pages/MigrationDetail'
import NewPlan from './pages/NewPlan'
import DriftScanPage from './pages/DriftScanPage'

export default function App() {
  return (
    <BrowserRouter>
      <ToastProvider>
        <ErrorBoundary>
          <div className="app">
            <a href="#main-content" className="skip-link">Skip to main content</a>
            <nav className="nav" role="navigation" aria-label="Main navigation">
              <NavLink to="/" className="nav-brand" end>
                <svg className="nav-logo" width="28" height="28" viewBox="0 0 32 32" aria-hidden="true">
                  <defs>
                    <linearGradient id="nav-g" x1="0" y1="0" x2="1" y2="1">
                      <stop offset="0%" stopColor="#818cf8"/>
                      <stop offset="100%" stopColor="#4f46e5"/>
                    </linearGradient>
                  </defs>
                  <path d="M16 1L29.856 9v18L16 31 2.144 27V9z" fill="url(#nav-g)"/>
                  <path d="M10 16.5l3.5 3.5L22 11.5" stroke="#fff" stroke-width="2.4" fill="none" strokeLinecap="round" strokeLinejoin="round"/>
                </svg>
                MSE
                <span className="nav-pulse" title="Engine online" aria-label="Engine online" />
              </NavLink>
              <div className="nav-links">
                <NavLink to="/" end>Migrations</NavLink>
                <NavLink to="/plans/new">New Plan</NavLink>
                <NavLink to="/drift-scan">Drift Scan</NavLink>
              </div>
              <div className="nav-right">
                <div className="nav-search" aria-label="Search (⌘K)">
                  <svg width="12" height="12" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" aria-hidden="true">
                    <circle cx="7" cy="7" r="5"/><path d="M11 11l3 3"/>
                  </svg>
                  Search...
                  <kbd>⌘K</kbd>
                </div>
                <a
                  href="https://github.com/iamyadavvikas/migration-safety-engine"
                  target="_blank"
                  rel="noopener noreferrer"
                  className="nav-github"
                  aria-label="View source code on GitHub"
                >
                  <svg width="14" height="14" viewBox="0 0 16 16" fill="currentColor" aria-hidden="true">
                    <path d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27.68 0 1.36.09 2 .27 1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.013 8.013 0 0016 8c0-4.42-3.58-8-8-8z"/>
                  </svg>
                  GitHub
                </a>
                <span className="nav-version" aria-label="Version 0.1.0">v0.1.0</span>
              </div>
            </nav>
            <main id="main-content" className="main" role="main">
              <Routes>
                <Route path="/" element={<Dashboard />} />
                <Route path="/migrations/:id" element={<MigrationDetail />} />
                <Route path="/plans/new" element={<NewPlan />} />
                <Route path="/drift-scan" element={<DriftScanPage />} />
                <Route path="*" element={<Navigate to="/" replace />} />
              </Routes>
            </main>
            <footer className="footer" role="contentinfo">
              <span>Migration Safety Engine</span>
              <span>SLO-gated canary migrations for Postgres</span>
            </footer>
          </div>
        </ErrorBoundary>
      </ToastProvider>
    </BrowserRouter>
  )
}
