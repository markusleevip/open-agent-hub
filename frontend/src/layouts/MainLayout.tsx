import { useEffect, useMemo, useState } from 'react'
import { Layout, Menu, Dropdown, Avatar, Select, theme, Space, Typography, App } from 'antd'
import { Outlet, useNavigate, useLocation } from 'react-router-dom'
import {
  DashboardOutlined,
  TeamOutlined,
  KeyOutlined,
  AppstoreOutlined,
  RobotOutlined,
  ReadOutlined,
  DatabaseOutlined,
  ToolOutlined,
  HistoryOutlined,
  LogoutOutlined,
  UserOutlined,
  ProjectOutlined,
  RocketOutlined,
  GlobalOutlined
} from '@ant-design/icons'
import type { MenuProps } from 'antd'
import { useAuthStore } from '@/store/auth'
import { authApi } from '@/api'
import { useTranslation } from 'react-i18next'

const { Header, Sider, Content } = Layout
const { Text } = Typography

const MainLayout = () => {
  const navigate = useNavigate()
  const location = useLocation()
  const { t, i18n } = useTranslation()
  const { token: authToken, user, workspace, workspaces, setAuth, setWorkspace, clear, role } = useAuthStore()
  const [collapsed, setCollapsed] = useState(false)
  const { message } = App.useApp()
  const {
    token: { colorBgContainer }
  } = theme.useToken()

  // Fetch fresh /me on every MainLayout mount to keep workspaces list up-to-date
  // (workspaces may change after creating a team or accepting an invitation; stale persisted values are unreliable)
  useEffect(() => {
    if (!authToken) return
    authApi
      .me()
      .then((data) => {
        setAuth({
          user: data.user,
          org: data.org,
          workspace: data.workspace,
          workspaces: data.workspaces,
          role: data.role
        })
      })
      .catch(() => {
        /* handled by interceptor */
      })
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  const menuItems: MenuProps['items'] = useMemo(
    () => [
      {
        key: '/dashboard',
        icon: <DashboardOutlined />,
        label: t('layout.dashboard')
      },
      {
        key: '/guide',
        icon: <RocketOutlined />,
        label: t('layout.guide')
      },
      {
        key: 'workspace-group',
        type: 'group',
        label: t('layout.workspaceGroup'),
        children: [
          {
            key: '/workspaces',
            icon: <AppstoreOutlined />,
            label: t('layout.workspaceManagement')
          },
          {
            key: '/members',
            icon: <TeamOutlined />,
            label: t('layout.memberManagement')
          },
          {
            key: '/projects',
            icon: <ProjectOutlined />,
            label: t('layout.projectManagement')
          },
          {
            key: '/tokens',
            icon: <KeyOutlined />,
            label: t('layout.mcpTokens')
          },
          {
            key: '/audit',
            icon: <HistoryOutlined />,
            label: t('layout.auditLogs')
          }
        ]
      },
      {
        key: 'context-hub',
        type: 'group',
        label: t('layout.contextHub'),
        children: [
          {
            key: '/context/global-rules',
            icon: <ReadOutlined />,
            label: t('layout.globalRules')
          },
          {
            key: '/context/project-rules',
            icon: <ReadOutlined />,
            label: t('layout.projectRules')
          },
          {
            key: '/context/output-preferences',
            icon: <UserOutlined />,
            label: t('layout.outputPreferences')
          }
        ]
      },
      {
        key: 'memory-hub',
        type: 'group',
        label: t('layout.memoryHub'),
        children: [
          {
            key: '/memory/explorer',
            icon: <DatabaseOutlined />,
            label: t('layout.memoryExplorer')
          },
          {
            key: '/memory/skills',
            icon: <DatabaseOutlined />,
            label: t('layout.skills')
          },
          {
            key: '/memory/public-skills',
            icon: <AppstoreOutlined />,
            label: t('layout.publicSkills')
          }
        ]
      },
      {
        key: 'tool-hub',
        type: 'group',
        label: t('layout.toolHub'),
        children: [
          {
            key: '/tool/connected-servers',
            icon: <ToolOutlined />,
            label: t('layout.connectedServers')
          },
          {
            key: '/tool/policies',
            icon: <ToolOutlined />,
            label: t('layout.toolPolicies')
          },
          {
            key: '/tool/invocations',
            icon: <HistoryOutlined />,
            label: t('layout.invocations')
          },
          {
            key: '/agent/clients',
            icon: <RobotOutlined />,
            label: t('layout.agentClients')
          },
          {
            key: '/tool/mcp-inspector',
            icon: <ToolOutlined />,
            label: t('layout.mcpInspector')
          }
        ]
      }
    ],
    [t]
  )

  const selectedKey = useMemo(() => {
    const path = location.pathname
    // Find exact match or best prefix match
    return path
  }, [location.pathname])

  const onWorkspaceChange = async (id: string) => {
    try {
      const data = await authApi.switchWorkspace(id)
      localStorage.setItem('oah_token', data.token)
      setAuth({
        token: data.token,
        user: data.user,
        org: data.org,
        workspace: data.workspace,
        workspaces: data.workspaces,
        role: data.role
      })
      setWorkspace(data.workspace)
      message.success(t('layout.switchSuccess', { name: data.workspace?.name }))
      // Reload page to refresh all data
      window.location.reload()
    } catch {
      // handled by interceptor
    }
  }

  const onLogout = () => {
    clear()
    localStorage.removeItem('oah_token')
    navigate('/login')
  }

  return (
    <Layout style={{ minHeight: '100vh' }}>
      <Sider
        collapsible
        collapsed={collapsed}
        onCollapse={setCollapsed}
        width={240}
        style={{ background: colorBgContainer, borderRight: '1px solid #f0f0f0' }}
        theme="light"
      >
        <div
          style={{
            height: 56,
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            fontWeight: 600,
            fontSize: collapsed ? 14 : 18,
            color: '#1677ff',
            borderBottom: '1px solid #f0f0f0'
          }}
        >
          {collapsed ? 'OAH' : '🪐 Open Agent Hub'}
        </div>
        <Menu
          mode="inline"
          selectedKeys={[selectedKey]}
          items={menuItems}
          onClick={({ key }) => navigate(key)}
          style={{ borderRight: 0, height: 'calc(100vh - 56px)', overflow: 'auto' }}
        />
      </Sider>
      <Layout>
        <Header
          style={{
            background: colorBgContainer,
            padding: '0 24px',
            borderBottom: '1px solid #f0f0f0',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between'
          }}
        >
          <Space size="middle">
            <Text type="secondary">{t('layout.currentWorkspace')}</Text>
            <Select
              value={workspace?.id}
              style={{ minWidth: 200 }}
              onChange={onWorkspaceChange}
              options={workspaces.map((ws) => ({
                value: ws.id,
                label: ws.type === 'personal' ? `👤 ${ws.name}` : `👥 ${ws.name}`
              }))}
            />
          </Space>
          <Space size="large">
            <Dropdown
              menu={{
                items: [
                  { key: 'zh', label: '简体中文' },
                  { key: 'en', label: 'English' }
                ],
                onClick: ({ key }) => {
                  i18n.changeLanguage(key)
                }
              }}
            >
              <Space style={{ cursor: 'pointer' }}>
                <GlobalOutlined />
                <Text>{i18n.language.startsWith('zh') ? '中文' : 'English'}</Text>
              </Space>
            </Dropdown>
            <Dropdown
              menu={{
                items: [
                  {
                    key: 'logout',
                    icon: <LogoutOutlined />,
                    label: t('layout.logout'),
                    onClick: onLogout
                  }
                ]
              }}
            >
              <Space style={{ cursor: 'pointer' }}>
                <Avatar size="small" icon={<UserOutlined />} />
                <Text>{user?.display_name ?? user?.username ?? t('layout.notLoggedIn')}</Text>
                {role && <Text type="secondary" style={{ fontSize: 12 }}>{role}</Text>}
              </Space>
            </Dropdown>
          </Space>
        </Header>
        <Content style={{ background: '#f5f7fb', overflow: 'auto' }}>
          <Outlet />
        </Content>
      </Layout>
    </Layout>
  )
}

export default MainLayout
