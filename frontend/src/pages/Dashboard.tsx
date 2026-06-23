import { useEffect, useState } from 'react'
import { Card, Col, Row, Statistic, Typography, Tag, Space, Spin, Progress, App, Table, Button, theme } from 'antd'
import {
  CloudServerOutlined,
  RobotOutlined,
  DatabaseOutlined,
  ToolOutlined,
  RiseOutlined,
  SafetyCertificateOutlined
} from '@ant-design/icons'
import { useAuthStore } from '@/store/auth'
import { usageApi, memoryApi, agentClientApi, connectedServerApi, toolInvocationApi, memberApi, authApi } from '@/api'
import type { UsageDashboard, AgentClient, ConnectedMCPServer, ToolInvocationLog, Invitation } from '@/types'
import { formatDate, shortId, truncate } from '@/utils'
import { useTranslation, Trans } from 'react-i18next'

const { Title, Text } = Typography

const DashboardPage = () => {
  const { t } = useTranslation()
  const { user, workspace, org, role, setAuth } = useAuthStore()
  const { message } = App.useApp()
  const { token } = theme.useToken()
  const [loading, setLoading] = useState(false)
  const [dashboard, setDashboard] = useState<UsageDashboard | null>(null)
  const [stats, setStats] = useState<Record<string, number>>({})
  const [agents, setAgents] = useState<AgentClient[]>([])
  const [servers, setServers] = useState<ConnectedMCPServer[]>([])
  const [recentInvocations, setRecentInvocations] = useState<ToolInvocationLog[]>([])
  const [invitations, setInvitations] = useState<Invitation[]>([])

  useEffect(() => {
    loadAll()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  const loadAll = async () => {
    setLoading(true)
    try {
      const [d, s, a, sv, ri, inv] = await Promise.allSettled([
        usageApi.dashboard(),
        memoryApi.stats(),
        agentClientApi.list(),
        connectedServerApi.list(),
        toolInvocationApi.list({ limit: 10 }),
        memberApi.myInvitations()
      ])
      if (d.status === 'fulfilled') setDashboard(d.value)
      if (s.status === 'fulfilled') setStats(s.value)
      if (a.status === 'fulfilled') setAgents(a.value)
      if (sv.status === 'fulfilled') setServers(sv.value)
      if (ri.status === 'fulfilled') setRecentInvocations(ri.value)
      if (inv.status === 'fulfilled') setInvitations(inv.value)
    } catch (e) {
      message.error(t('dashboard.loadError'))
    } finally {
      setLoading(false)
    }
  }

  const onAccept = async (inv: Invitation) => {
    await memberApi.acceptInvitation(inv.id)
    const data = await authApi.switchWorkspace(inv.workspace_id)
    localStorage.setItem('oah_token', data.token)
    setAuth({
      token: data.token,
      user: data.user,
      org: data.org,
      workspace: data.workspace,
      workspaces: data.workspaces,
      role: data.role
    })
    message.success(t('dashboard.invitationAccepted'))
    loadAll()
  }

  const onReject = async (id: string) => {
    await memberApi.rejectInvitation(id)
    message.success(t('dashboard.invitationRejected'))
    loadAll()
  }

  const memQuotaPct = workspace?.quota_memory_count
    ? Math.min(100, ((stats.total_count ?? dashboard?.memory_total ?? 0) / workspace.quota_memory_count) * 100)
    : 0
  const callQuotaPct = workspace?.quota_tool_call_daily
    ? Math.min(
        100,
        ((dashboard?.today_calls ?? 0) /
          workspace.quota_tool_call_daily) *
          100
      )
    : 0

  return (
    <div className="page-container">
      <Spin spinning={loading}>
        <Space direction="vertical" size="middle" style={{ width: '100%' }}>
          <Card>
            <Space direction="vertical" size={4}>
              <Title level={3} style={{ margin: 0 }}>
                {t('dashboard.welcome', { name: user?.display_name || user?.username })}
              </Title>
              <Text type="secondary">
                <Trans
                  i18nKey="dashboard.accessStatus"
                  values={{ org: org?.name ?? '-', workspace: workspace?.name ?? '-', role }}
                  components={[
                    <Tag color="blue" key="org" />,
                    <Tag color="green" key="ws" />,
                    <Tag color="purple" key="role" />
                  ]}
                />
              </Text>
            </Space>
          </Card>

          <Row gutter={[16, 16]}>
            <Col xs={24} sm={12} md={6}>
              <Card>
                <Statistic
                  title={t('dashboard.todayCalls')}
                  value={dashboard?.today_calls ?? 0}
                  prefix={<ToolOutlined />}
                  valueStyle={{ color: token.colorPrimary }}
                />
                <Text type="secondary" style={{ fontSize: 12 }}>
                  {t('dashboard.monthAndSessions', {
                    monthCalls: dashboard?.month_calls ?? 0,
                    sessions: dashboard?.active_sessions ?? 0
                  })}
                </Text>
              </Card>
            </Col>
            <Col xs={24} sm={12} md={6}>
              <Card>
                <Statistic
                  title={t('dashboard.totalMemories')}
                  value={stats.total_count ?? dashboard?.memory_total ?? 0}
                  prefix={<DatabaseOutlined />}
                  valueStyle={{ color: '#52c41a' }}
                />
                <Text type="secondary" style={{ fontSize: 12 }}>
                  {t('dashboard.writesAndQuota', {
                    writes: dashboard?.memory_writes ?? 0,
                    quota: workspace?.quota_memory_count ?? 0
                  })}
                </Text>
              </Card>
            </Col>
            <Col xs={24} sm={12} md={6}>
              <Card>
                <Statistic
                  title={t('dashboard.activeAgents')}
                  value={agents.length}
                  prefix={<RobotOutlined />}
                  valueStyle={{ color: '#722ed1' }}
                />
                <Text type="secondary" style={{ fontSize: 12 }}>
                  {t('dashboard.autoDetect')}
                </Text>
              </Card>
            </Col>
            <Col xs={24} sm={12} md={6}>
              <Card>
                <Statistic
                  title={t('dashboard.connectedServers')}
                  value={servers.length}
                  prefix={<CloudServerOutlined />}
                  valueStyle={{ color: '#fa8c16' }}
                />
                <Text type="secondary" style={{ fontSize: 12 }}>
                  {t('dashboard.serversStatus', {
                    active: servers.filter((s) => s.status === 'active').length,
                    pending: servers.filter((s) => s.status === 'pending').length
                  })}
                </Text>
              </Card>
            </Col>
          </Row>

          <Row gutter={[16, 16]}>
            <Col xs={24} md={12}>
              <Card title={<><RiseOutlined /> {t('dashboard.memoryQuota')}</>}>
                <Progress
                  percent={Math.round(memQuotaPct)}
                  status={memQuotaPct > 80 ? 'exception' : 'active'}
                />
                <Text type="secondary" style={{ fontSize: 12 }}>
                  {stats.active ?? stats.total_count ?? 0} / {workspace?.quota_memory_count ?? 0}
                </Text>
              </Card>
            </Col>
            <Col xs={24} md={12}>
              <Card title={<><SafetyCertificateOutlined /> {t('dashboard.todayCallsQuota')}</>}>
                <Progress
                  percent={Math.round(callQuotaPct)}
                  status={callQuotaPct > 80 ? 'exception' : 'active'}
                />
                <Text type="secondary" style={{ fontSize: 12 }}>
                  {dashboard?.today_calls ?? 0} / {workspace?.quota_tool_call_daily ?? 0}
                </Text>
              </Card>
            </Col>
          </Row>

          {invitations.length > 0 && (
            <Card title={t('dashboard.myInvitations')}>
              {invitations.map((inv) => (
                <div key={inv.id} style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '8px 0', borderBottom: '1px solid #f0f0f0' }}>
                  <Space>
                    <Tag color="purple">{inv.role}</Tag>
                    <Text strong>{inv.workspace?.name ?? '-'}</Text>
                    <Text type="secondary">{t('dashboard.invitationDesc', { date: formatDate(inv.invited_at) })}</Text>
                  </Space>
                  <Space>
                    <Button type="primary" size="small" onClick={() => onAccept(inv)}>
                      {t('dashboard.acceptInvitation')}
                    </Button>
                    <Button size="small" onClick={() => onReject(inv.id)}>
                      {t('dashboard.rejectInvitation')}
                    </Button>
                  </Space>
                </div>
              ))}
            </Card>
          )}

          <Card title={t('dashboard.recentInvocations')}>
            <Table
              size="small"
              rowKey="id"
              dataSource={recentInvocations}
              pagination={false}
              columns={[
                {
                  title: t('dashboard.time'),
                  dataIndex: 'invoked_at',
                  width: 170,
                  render: (v) => formatDate(v)
                },
                {
                  title: t('dashboard.tool'),
                  dataIndex: 'tool_name',
                  render: (v) => <code>{v}</code>
                },
                {
                  title: t('dashboard.status'),
                  dataIndex: 'status',
                  width: 100,
                  render: (v) => (
                    <Tag color={v === 'success' ? 'green' : 'red'}>{v}</Tag>
                  )
                },
                {
                  title: t('dashboard.latency'),
                  dataIndex: 'latency_ms',
                  width: 90,
                  render: (v) => `${v} ms`
                },
                {
                  title: t('dashboard.session'),
                  dataIndex: 'mcp_session_id',
                  width: 110,
                  render: (v) => <code>{shortId(v, 8)}</code>
                },
                {
                  title: t('dashboard.outputSummary'),
                  dataIndex: 'output_summary',
                  ellipsis: true,
                  render: (v) => <Text type="secondary">{truncate(v, 80)}</Text>
                }
              ]}
              locale={{ emptyText: t('dashboard.emptyTableHint') }}
            />
          </Card>
        </Space>
      </Spin>
    </div>
  )
}

export default DashboardPage

