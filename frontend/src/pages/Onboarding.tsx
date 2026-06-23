import { useEffect, useState } from 'react'
import { Card, Form, Input, Button, App, Typography, List, Tag, Empty, Space, Divider } from 'antd'
import { TeamOutlined, MailOutlined, LogoutOutlined } from '@ant-design/icons'
import { useNavigate } from 'react-router-dom'
import { workspaceApi, memberApi, authApi } from '@/api'
import { useAuthStore } from '@/store/auth'
import type { Invitation } from '@/types'
import { formatDate } from '@/utils'
import { useTranslation } from 'react-i18next'

const { Title, Text } = Typography

const OnboardingPage = () => {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { message } = App.useApp()
  const { user, setAuth, clear } = useAuthStore()
  const [creating, setCreating] = useState(false)
  const [invitations, setInvitations] = useState<Invitation[]>([])
  const [loadingInv, setLoadingInv] = useState(false)
  const [actingId, setActingId] = useState<string | null>(null)

  const loadInvitations = async () => {
    setLoadingInv(true)
    try {
      const list = await memberApi.myInvitations()
      setInvitations(list || [])
    } finally {
      setLoadingInv(false)
    }
  }

  useEffect(() => {
    loadInvitations()
  }, [])

  const adopt = (data: Awaited<ReturnType<typeof workspaceApi.create>>) => {
    if (data.token) localStorage.setItem('oah_token', data.token)
    setAuth({
      token: data.token,
      user: data.user,
      org: data.org,
      workspace: data.workspace,
      workspaces: data.workspaces,
      role: data.role
    })
    navigate('/dashboard')
  }

  const onCreate = async (values: { name: string }) => {
    setCreating(true)
    try {
      const data = await workspaceApi.create({ name: values.name })
      message.success(t('onboarding.createSuccess', { name: data.workspace?.name }))
      adopt(data)
    } catch {
      /* interceptor */
    } finally {
      setCreating(false)
    }
  }

  const onAccept = async (inv: Invitation) => {
    setActingId(inv.id)
    try {
      await memberApi.acceptInvitation(inv.id)
      // After accepting, the current token still lacks team context; switch into the team to get a new token
      const data = await authApi.switchWorkspace(inv.workspace_id)
      message.success(t('onboarding.acceptSuccess', { name: data.workspace?.name }))
      adopt(data)
    } catch {
      /* interceptor */
    } finally {
      setActingId(null)
    }
  }

  const onReject = async (inv: Invitation) => {
    setActingId(inv.id)
    try {
      await memberApi.rejectInvitation(inv.id)
      message.success(t('onboarding.rejectSuccess'))
      loadInvitations()
    } catch {
      /* interceptor */
    } finally {
      setActingId(null)
    }
  }

  return (
    <div style={{ maxWidth: 560, margin: '48px auto', padding: '0 16px' }}>
      <Space direction="vertical" size="middle" style={{ width: '100%' }}>
        <div style={{ textAlign: 'center' }}>
          <Title level={3} style={{ marginBottom: 4 }}>
            {t('onboarding.title', { name: user?.display_name || user?.username })}
          </Title>
          <Text type="secondary">{t('onboarding.subtitle')}</Text>
        </div>

        <Card title={<Space><TeamOutlined />{t('onboarding.createTitle')}</Space>}>
          <Text type="secondary" style={{ display: 'block', marginBottom: 12 }}>
            {t('onboarding.createDesc')}
          </Text>
          <Form layout="vertical" onFinish={onCreate}>
            <Form.Item
              name="name"
              label={t('onboarding.teamName')}
              rules={[{ required: true, message: t('onboarding.teamNameRequired') }]}
            >
              <Input placeholder={t('onboarding.teamNamePlaceholder')} maxLength={64} />
            </Form.Item>
            <Button type="primary" htmlType="submit" block loading={creating}>
              {t('onboarding.createBtn')}
            </Button>
          </Form>
        </Card>

        <Card title={<Space><MailOutlined />{t('onboarding.inviteTitle')}</Space>} loading={loadingInv}>
          {invitations.length === 0 ? (
            <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={t('onboarding.noInvitations')} />
          ) : (
            <List
              dataSource={invitations}
              renderItem={(inv) => (
                <List.Item
                  actions={[
                    <Button
                      key="accept"
                      type="link"
                      loading={actingId === inv.id}
                      onClick={() => onAccept(inv)}
                    >
                      {t('onboarding.accept')}
                    </Button>,
                    <Button
                      key="reject"
                      type="link"
                      danger
                      loading={actingId === inv.id}
                      onClick={() => onReject(inv)}
                    >
                      {t('onboarding.reject')}
                    </Button>
                  ]}
                >
                  <List.Item.Meta
                    title={inv.workspace?.name || inv.workspace_id}
                    description={
                      <Space size="small">
                        <Tag color="blue">{inv.role}</Tag>
                        <Text type="secondary" style={{ fontSize: 12 }}>
                          {t('onboarding.invitedAt')}: {formatDate(inv.invited_at)}
                        </Text>
                      </Space>
                    }
                  />
                </List.Item>
              )}
            />
          )}
        </Card>

        <Divider style={{ margin: '8px 0' }} />
        <Button type="text" icon={<LogoutOutlined />} onClick={() => { clear(); navigate('/login') }}>
          {t('onboarding.logout')}
        </Button>
      </Space>
    </div>
  )
}

export default OnboardingPage
