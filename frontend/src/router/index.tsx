import { createBrowserRouter, Navigate } from 'react-router-dom'
import { lazy, Suspense } from 'react'
import { Spin } from 'antd'
import MainLayout from '@/layouts/MainLayout'
import LoginPage from '@/pages/Login'
import OnboardingPage from '@/pages/Onboarding'
import { useAuthStore } from '@/store/auth'

const DashboardPage = lazy(() => import('@/pages/Dashboard'))
const GettingStartedPage = lazy(() => import('@/pages/GettingStarted'))
const WorkspacesPage = lazy(() => import('@/pages/Workspaces'))
const MembersPage = lazy(() => import('@/pages/Members'))
const TokensPage = lazy(() => import('@/pages/Tokens'))
const ProjectsPage = lazy(() => import('@/pages/Projects'))
const ProjectDetailPage = lazy(() => import('@/pages/ProjectDetail'))
const AuditPage = lazy(() => import('@/pages/Audit'))

// Agent Hub
const AgentClientsPage = lazy(() => import('@/pages/agent/AgentClients'))

// Context Hub
const GlobalRulesPage = lazy(() => import('@/pages/context/GlobalRules'))
const ProjectRulesPage = lazy(() => import('@/pages/context/ProjectRules'))
const OutputPreferencesPage = lazy(() => import('@/pages/context/OutputPreferences'))

// Memory Hub
const MemoryExplorerPage = lazy(() => import('@/pages/memory/MemoryExplorer'))
const SkillsPage = lazy(() => import('@/pages/memory/Skills'))
const PublicSkillsPage = lazy(() => import('@/pages/memory/PublicSkills'))

// Tool Hub
const ConnectedServersPage = lazy(() => import('@/pages/tool/ConnectedServers'))
const ToolPoliciesPage = lazy(() => import('@/pages/tool/ToolPolicies'))
const ToolInvocationsPage = lazy(() => import('@/pages/tool/ToolInvocations'))
const MCPInspectorPage = lazy(() => import('@/pages/tool/MCPInspector'))

const SuspenseWrap = ({ children }: { children: React.ReactNode }) => (
  <Suspense
    fallback={
      <div style={{ padding: 48, textAlign: 'center' }}>
        <Spin tip="Loading..." />
      </div>
    }
  >
    {children}
  </Suspense>
)

const ProtectedRoute = ({ children }: { children: React.ReactNode }) => {
  const token = useAuthStore((s) => s.token)
  if (!token) {
    return <Navigate to="/login" replace />
  }
  return <>{children}</>
}

// Requires workspace membership: users without a team are redirected to onboarding
const WorkspaceRoute = ({ children }: { children: React.ReactNode }) => {
  const token = useAuthStore((s) => s.token)
  const workspace = useAuthStore((s) => s.workspace)
  if (!token) {
    return <Navigate to="/login" replace />
  }
  if (!workspace) {
    return <Navigate to="/onboarding" replace />
  }
  return <>{children}</>
}

const AdminRoute = ({ children }: { children: React.ReactNode }) => {
  const token = useAuthStore((s) => s.token)
  const role = useAuthStore((s) => s.role)
  if (!token) {
    return <Navigate to="/login" replace />
  }
  if (role !== 'owner' && role !== 'admin') {
    return <Navigate to="/dashboard" replace />
  }
  return <>{children}</>
}

export const router = createBrowserRouter([
  {
    path: '/login',
    element: <LoginPage />
  },
  {
    path: '/onboarding',
    element: (
      <ProtectedRoute>
        <OnboardingPage />
      </ProtectedRoute>
    )
  },
  {
    path: '/',
    element: (
      <WorkspaceRoute>
        <MainLayout />
      </WorkspaceRoute>
    ),
    children: [
      { index: true, element: <Navigate to="/dashboard" replace /> },
      {
        path: 'dashboard',
        element: (
          <SuspenseWrap>
            <DashboardPage />
          </SuspenseWrap>
        )
      },
      {
        path: 'guide',
        element: (
          <SuspenseWrap>
            <GettingStartedPage />
          </SuspenseWrap>
        )
      },
      {
        path: 'workspaces',
        element: (
          <SuspenseWrap>
            <WorkspacesPage />
          </SuspenseWrap>
        )
      },
      {
        path: 'members',
        element: (
          <SuspenseWrap>
            <MembersPage />
          </SuspenseWrap>
        )
      },
      {
        path: 'projects',
        element: (
          <SuspenseWrap>
            <ProjectsPage />
          </SuspenseWrap>
        )
      },
      {
        path: 'projects/:id',
        element: (
          <SuspenseWrap>
            <ProjectDetailPage />
          </SuspenseWrap>
        )
      },
      {
        path: 'tokens',
        element: (
          <SuspenseWrap>
            <TokensPage />
          </SuspenseWrap>
        )
      },
      {
        path: 'audit',
        element: (
          <SuspenseWrap>
            <AuditPage />
          </SuspenseWrap>
        )
      },
      // Agent Hub
      {
        path: 'agent/clients',
        element: (
          <SuspenseWrap>
            <AgentClientsPage />
          </SuspenseWrap>
        )
      },
      // Context Hub
      {
        path: 'context/global-rules',
        element: (
          <SuspenseWrap>
            <GlobalRulesPage />
          </SuspenseWrap>
        )
      },
      {
        path: 'context/project-rules',
        element: (
          <SuspenseWrap>
            <ProjectRulesPage />
          </SuspenseWrap>
        )
      },
      {
        path: 'context/output-preferences',
        element: (
          <SuspenseWrap>
            <OutputPreferencesPage />
          </SuspenseWrap>
        )
      },
      // Memory Hub
      {
        path: 'memory/explorer',
        element: (
          <SuspenseWrap>
            <MemoryExplorerPage />
          </SuspenseWrap>
        )
      },
      {
        path: 'memory/skills',
        element: (
          <SuspenseWrap>
            <SkillsPage />
          </SuspenseWrap>
        )
      },
      {
        path: 'memory/public-skills',
        element: (
          <SuspenseWrap>
            <PublicSkillsPage />
          </SuspenseWrap>
        )
      },
      // Tool Hub
      {
        path: 'tool/connected-servers',
        element: (
          <SuspenseWrap>
            <ConnectedServersPage />
          </SuspenseWrap>
        )
      },
      {
        path: 'tool/policies',
        element: (
          <SuspenseWrap>
            <ToolPoliciesPage />
          </SuspenseWrap>
        )
      },
      {
        path: 'tool/invocations',
        element: (
          <SuspenseWrap>
            <ToolInvocationsPage />
          </SuspenseWrap>
        )
      },
      {
        path: 'tool/mcp-inspector',
        element: (
          <SuspenseWrap>
            <MCPInspectorPage />
          </SuspenseWrap>
        )
      }
    ]
  },
  {
    path: '*',
    element: <Navigate to="/dashboard" replace />
  }
])
