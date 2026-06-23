import { useEffect, useState } from 'react'
import { Card, Table, Button, Modal, Form, Input, App, Space, Tag, Typography, Popconfirm } from 'antd'
import { DeleteOutlined, EditOutlined, LogoutOutlined, PlusOutlined } from '@ant-design/icons'
import { useNavigate } from 'react-router-dom'
import { workspaceApi, authApi } from '@/api'
import type { Workspace } from '@/types'
import { useAuthStore } from '@/store/auth'
import { formatDate, shortId } from '@/utils'
import { useTranslation } from 'react-i18next'

const { Title, Text } = Typography

type WorkspaceRow = Workspace & { role?: string }

const WorkspacesPage = () => {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const user = useAuthStore((s) => s.user)
  const workspace = useAuthStore((s) => s.workspace)
  const setAuth = useAuthStore((s) => s.setAuth)
  const { message } = App.useApp()
  const [data, setData] = useState<WorkspaceRow[]>([])
  const [loading, setLoading] = useState(false)
  const [open, setOpen] = useState(false)
  const [createOpen, setCreateOpen] = useState(false)
  const [editing, setEditing] = useState<Workspace | null>(null)
  const [form] = Form.useForm()
  const [createForm] = Form.useForm()

  useEffect(() => {
    load()
  }, [])

  const load = async () => {
    setLoading(true)
    try {
      const list = (await workspaceApi.list()) as WorkspaceRow[]
      setData(list)
    } finally {
      setLoading(false)
    }
  }

  const onEdit = (row: Workspace) => {
    setEditing(row)
    form.setFieldsValue(row)
    setOpen(true)
  }

  const onSubmit = async () => {
    const values = await form.validateFields()
    if (editing) {
      await workspaceApi.update(editing.id, values)
      message.success(t('workspaces.updated'))
    }
    setOpen(false)
    load()
  }

  // After deleting/leaving a team, try to switch to personal workspace
  const afterLeaveOrDelete = async () => {
    try {
      const list = (await workspaceApi.list()) as WorkspaceRow[]
      const personal = list.find((w) => w.type === 'personal')
      if (personal) {
        const data = await authApi.switchWorkspace(personal.id)
        localStorage.setItem('oah_token', data.token)
        setAuth({
          token: data.token,
          user: data.user,
          org: data.org,
          workspace: data.workspace,
          workspaces: data.workspaces,
          role: data.role
        })
        window.location.reload()
      } else {
        setAuth({ user, org: null, workspace: null, workspaces: [], role: '' })
        navigate('/onboarding')
      }
    } catch {
      setAuth({ user, org: null, workspace: null, workspaces: [], role: '' })
      navigate('/onboarding')
    }
  }

  const onDelete = async (id: string) => {
    await workspaceApi.delete(id)
    message.success(t('workspaces.deleted'))
    afterLeaveOrDelete()
  }

  const onLeave = async (id: string) => {
    await workspaceApi.leave(id)
    message.success(t('workspaces.left'))
    afterLeaveOrDelete()
  }

  const onSwitch = async (id: string) => {
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
    message.success(t('workspaces.switchSuccess', { name: data.workspace?.name }))
    load()
  }

  const onCreateTeam = async () => {
    const values = await createForm.validateFields()
    const data = await workspaceApi.create({ name: values.name })
    if (data.token) localStorage.setItem('oah_token', data.token)
    setAuth({
      token: data.token,
      user: data.user,
      org: data.org,
      workspace: data.workspace,
      workspaces: data.workspaces,
      role: data.role
    })
    message.success(t('workspaces.created'))
    setCreateOpen(false)
    createForm.resetFields()
    window.location.reload()
  }

  return (
    <div className="page-container">
      <Card
        title={<Title level={4} style={{ margin: 0 }}>{t('workspaces.title')}</Title>}
        extra={
          <Button type="primary" icon={<PlusOutlined />} onClick={() => setCreateOpen(true)}>
            {t('workspaces.createTeam')}
          </Button>
        }
      >
        <Text type="secondary" style={{ display: 'block', marginBottom: 12 }}>
          {t('workspaces.desc')}
        </Text>
        <Table
          rowKey="id"
          loading={loading}
          dataSource={data}
          pagination={false}
          columns={[
            {
              title: 'ID',
              dataIndex: 'id',
              width: 110,
              render: (v) => <code>{shortId(v, 8)}</code>
            },
            { title: t('workspaces.name'), dataIndex: 'name', render: (v, r) => (
              <Space>
                <Text strong>{v}</Text>
                {r.id === workspace?.id && <Tag color="green">{t('workspaces.current')}</Tag>}
              </Space>
            ) },
            { title: t('workspaces.type'), dataIndex: 'type', width: 100, render: (v) => <Tag color={v === 'personal' ? 'cyan' : 'blue'}>{v === 'personal' ? t('workspaces.personal') : t('workspaces.team')}</Tag> },
            { title: t('workspaces.slug'), dataIndex: 'slug', render: (v) => <code>{v}</code> },
            {
              title: t('workspaces.quota'),
              render: (_, r) => (
                <Space size="small" direction="vertical">
                  <Text type="secondary" style={{ fontSize: 12 }}>
                    Memory: {r.quota_memory_count}
                  </Text>
                  <Text type="secondary" style={{ fontSize: 12 }}>
                    Tool/Day: {r.quota_tool_call_daily}
                  </Text>
                </Space>
              )
            },
            {
              title: t('workspaces.status'),
              dataIndex: 'status',
              width: 90,
              render: (v) => <Tag color={v === 'active' ? 'green' : 'default'}>{v}</Tag>
            },
            {
              title: t('workspaces.createdAt'),
              dataIndex: 'created_at',
              width: 170,
              render: (v) => formatDate(v)
            },
            {
              title: t('workspaces.role'),
              dataIndex: 'role',
              width: 100,
              render: (v) => <Tag color={v === 'owner' ? 'gold' : 'blue'}>{v}</Tag>
            },
            {
              title: t('workspaces.actions'),
              width: 280,
              render: (_, r) => {
                const isCurrent = r.id === workspace?.id
                return (
                  <Space>
                    {!isCurrent && (
                      <Button type="link" onClick={() => onSwitch(r.id)}>
                        {t('workspaces.switch')}
                      </Button>
                    )}
                    {r.role === 'owner' ? (
                      <>
                        <Button type="link" icon={<EditOutlined />} onClick={() => onEdit(r)}>
                          {t('workspaces.edit')}
                        </Button>
                        {r.type !== 'personal' && (
                          <Popconfirm
                            title={t('workspaces.confirmDelete')}
                            description={t('workspaces.deleteWarn')}
                            onConfirm={() => onDelete(r.id)}
                          >
                            <Button type="link" danger icon={<DeleteOutlined />}>
                              {t('workspaces.delete')}
                            </Button>
                          </Popconfirm>
                        )}
                      </>
                    ) : (
                      <Popconfirm
                        title={t('workspaces.confirmLeave')}
                        description={t('workspaces.leaveWarn')}
                        onConfirm={() => onLeave(r.id)}
                      >
                        <Button type="link" danger icon={<LogoutOutlined />}>
                          {t('workspaces.leave')}
                        </Button>
                      </Popconfirm>
                    )}
                  </Space>
                )
              }
            }
          ]}
        />
      </Card>

      <Modal
        title={t('workspaces.editWorkspace')}
        open={open}
        onCancel={() => setOpen(false)}
        onOk={onSubmit}
        destroyOnClose
      >
        <Form form={form} layout="vertical">
          <Form.Item name="name" label={t('workspaces.name')} rules={[{ required: true, message: t('workspaces.name') }]}>
            <Input placeholder={t('workspaces.namePlaceholder')} />
          </Form.Item>
          <Form.Item name="slug" label={t('workspaces.slug')}>
            <Input placeholder={t('workspaces.slugPlaceholder')} />
          </Form.Item>
          <Form.Item name="quota_memory_count" label={t('workspaces.memoryQuota')}>
            <Input type="number" placeholder={t('workspaces.memoryQuotaPlaceholder')} />
          </Form.Item>
          <Form.Item name="quota_tool_call_daily" label={t('workspaces.dailyCallsQuota')}>
            <Input type="number" placeholder={t('workspaces.dailyCallsQuotaPlaceholder')} />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        title={t('workspaces.createTeam')}
        open={createOpen}
        onCancel={() => setCreateOpen(false)}
        onOk={onCreateTeam}
        destroyOnClose
      >
        <Form form={createForm} layout="vertical">
          <Form.Item name="name" label={t('workspaces.name')} rules={[{ required: true, message: t('workspaces.nameRequired') }]}>
            <Input placeholder={t('workspaces.namePlaceholder')} />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}

export default WorkspacesPage
