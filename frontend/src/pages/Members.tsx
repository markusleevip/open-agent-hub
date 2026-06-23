import { useEffect, useState } from 'react'
import { Card, Table, Button, Modal, Form, Input, Select, App, Tag, Typography, Popconfirm, Alert } from 'antd'
import { UserAddOutlined, DeleteOutlined } from '@ant-design/icons'
import { memberApi } from '@/api'
import type { WorkspaceMember } from '@/types'
import { useAuthStore } from '@/store/auth'
import { formatDate, shortId } from '@/utils'
import { useTranslation } from 'react-i18next'

const { Title, Text } = Typography

const roleColor: Record<string, string> = {
  owner: 'gold',
  admin: 'purple',
  member: 'blue',
  viewer: 'default'
}

const MembersPage = () => {
  const { t } = useTranslation()
  const { message } = App.useApp()
  const myRole = useAuthStore((s) => s.role)
  const workspace = useAuthStore((s) => s.workspace)
  const isPersonal = workspace?.type === 'personal'
  const [data, setData] = useState<WorkspaceMember[]>([])
  const [loading, setLoading] = useState(false)
  const [open, setOpen] = useState(false)
  const [form] = Form.useForm()

  useEffect(() => {
    load()
  }, [])

  const load = async () => {
    setLoading(true)
    try {
      const list = await memberApi.list()
      setData(list)
    } finally {
      setLoading(false)
    }
  }

  const onInvite = async () => {
    const values = await form.validateFields()
    await memberApi.invite(values)
    message.success(t('members.invited'))
    setOpen(false)
    load()
  }

  const onChangeRole = async (id: string, role: string) => {
    await memberApi.updateRole(id, role)
    message.success(t('members.roleUpdated'))
    load()
  }

  const onRemove = async (id: string) => {
    await memberApi.remove(id)
    message.success(t('members.removed'))
    load()
  }

  const canManage = myRole === 'owner' || myRole === 'admin'

  // Owner rows are untouchable; admins can only manage member/viewer, not other admins
  const canManageMember = (r: WorkspaceMember) => {
    if (r.role === 'owner') return false
    if (myRole === 'owner') return true
    if (myRole === 'admin') return r.role !== 'admin'
    return false
  }

  // Assignable roles: owner can assign admin/member/viewer; admin can only assign member/viewer; owner is not assignable
  const roleOptions =
    myRole === 'owner'
      ? [
          { value: 'admin', label: 'admin' },
          { value: 'member', label: 'member' },
          { value: 'viewer', label: 'viewer' }
        ]
      : [
          { value: 'member', label: 'member' },
          { value: 'viewer', label: 'viewer' }
        ]

  return (
    <div className="page-container">
      <Card
        title={<Title level={4} style={{ margin: 0 }}>{t('members.title')}</Title>}
        extra={
          canManage && !isPersonal && (
            <Button type="primary" icon={<UserAddOutlined />} onClick={() => setOpen(true)}>
              {t('members.invite')}
            </Button>
          )
        }
      >
        {isPersonal ? (
          <Alert type="info" showIcon message={t('members.personalNoMembers')} />
        ) : (
        <>
        <Text type="secondary" style={{ display: 'block', marginBottom: 12 }}>
          {t('members.desc')}
        </Text>
        <Table
          rowKey="id"
          loading={loading}
          dataSource={data}
          pagination={{ pageSize: 20 }}
          columns={[
            {
              title: 'ID',
              dataIndex: 'id',
              width: 110,
              render: (v) => <code>{shortId(v)}</code>
            },
            {
              title: t('members.username'),
              render: (_, r) => r.user?.username ?? `user:${shortId(r.user_id)}`
            },
            {
              title: t('members.displayName'),
              render: (_, r) => r.user?.display_name ?? '-'
            },
            {
              title: t('members.role'),
              dataIndex: 'role',
              width: 180,
              render: (v, r) =>
                canManageMember(r) && r.status === 'active' ? (
                  <Select
                    value={v}
                    style={{ width: 140 }}
                    size="small"
                    options={roleOptions}
                    onChange={(nv) => onChangeRole(r.id, nv)}
                  />
                ) : (
                  <Tag color={roleColor[v] ?? 'default'}>{v}</Tag>
                )
            },
            {
              title: t('members.status'),
              dataIndex: 'status',
              width: 100,
              render: (v) => (
                <Tag color={v === 'active' ? 'green' : 'orange'}>
                  {v === 'active' ? t('members.status_active') : t('members.status_pending')}
                </Tag>
              )
            },
            {
              title: t('members.invitedAt'),
              dataIndex: 'invited_at',
              width: 170,
              render: (v) => formatDate(v)
            },
            {
              title: t('members.joinedAt'),
              dataIndex: 'joined_at',
              width: 170,
              render: (v) => formatDate(v)
            },
            {
              title: t('members.actions'),
              width: 100,
              render: (_, r) =>
                canManageMember(r) ? (
                  <Popconfirm title={t('members.confirmRemove')} onConfirm={() => onRemove(r.id)}>
                    <Button type="link" danger icon={<DeleteOutlined />}>
                      {t('members.remove')}
                    </Button>
                  </Popconfirm>
                ) : (
                  <Text type="secondary">-</Text>
                )
            }
          ]}
        />
        </>
      )}
      </Card>

      <Modal
        title={t('members.invite')}
        open={open}
        onCancel={() => setOpen(false)}
        onOk={onInvite}
        destroyOnClose
      >
        <Form form={form} layout="vertical" initialValues={{ role: 'member' }}>
          <Form.Item
            name="username"
            label={t('members.username')}
            rules={[
              { required: true, message: t('members.usernameRequired') },
              { min: 2, message: t('members.usernameInvalid') },
              { pattern: /^[a-zA-Z\u4e00-\u9fff][a-zA-Z0-9\u4e00-\u9fff]*$/, message: t('members.usernameInvalid') }
            ]}
          >
            <Input placeholder={t('members.usernamePlaceholder')} />
          </Form.Item>
          <Form.Item name="role" label={t('members.role')} rules={[{ required: true }]}>
            <Select
              options={[
                { value: 'admin', label: t('members.roleAdmin') },
                { value: 'member', label: t('members.roleMember') },
                { value: 'viewer', label: t('members.roleViewer') }
              ]}
            />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}

export default MembersPage
