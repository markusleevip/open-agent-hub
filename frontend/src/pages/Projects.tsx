import { useEffect, useState } from 'react'
import { Card, Table, Button, Modal, Form, Input, App, Typography, Popconfirm, Tag } from 'antd'
import { PlusOutlined, DeleteOutlined, EditOutlined } from '@ant-design/icons'
import { projectApi } from '@/api'
import type { Project } from '@/types'
import { formatDate, shortId } from '@/utils'
import { useTranslation } from 'react-i18next'
import { useNavigate } from 'react-router-dom'

const { Title, Text } = Typography
const { TextArea } = Input

const ProjectsPage = () => {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { message } = App.useApp()
  const [data, setData] = useState<Project[]>([])
  const [loading, setLoading] = useState(false)
  const [open, setOpen] = useState(false)
  const [editing, setEditing] = useState<Project | null>(null)
  const [form] = Form.useForm()

  useEffect(() => {
    load()
  }, [])

  const load = async () => {
    setLoading(true)
    try {
      const list = await projectApi.list()
      setData(list)
    } finally {
      setLoading(false)
    }
  }

  const onNew = () => {
    setEditing(null)
    form.resetFields()
    setOpen(true)
  }

  const onEdit = (row: Project) => {
    setEditing(row)
    form.setFieldsValue(row)
    setOpen(true)
  }

  const onSubmit = async () => {
    const values = await form.validateFields()
    if (editing) {
      await projectApi.update(editing.id, values)
      message.success(t('projects.updated'))
    } else {
      await projectApi.create(values)
      message.success(t('projects.created'))
    }
    setOpen(false)
    load()
  }

  const onDelete = async (id: string) => {
    await projectApi.delete(id)
    message.success(t('projects.deleted'))
    load()
  }

  return (
    <div className="page-container">
      <Card
        title={<Title level={4} style={{ margin: 0 }}>{t('projects.title')}</Title>}
        extra={
          <Button type="primary" icon={<PlusOutlined />} onClick={onNew}>
            {t('projects.create')}
          </Button>
        }
      >
        <Text type="secondary" style={{ display: 'block', marginBottom: 12 }}>
          {t('projects.desc')}
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
              title: t('projects.name'),
              dataIndex: 'name',
              render: (v, r) => (
                <Button type="link" style={{ padding: 0 }} onClick={() => navigate(`/projects/${r.id}`)}>
                  {v}
                </Button>
              )
            },
            { title: t('projects.slug'), dataIndex: 'slug', render: (v) => <code>{v}</code> },
            {
              title: t('projects.gitRemote'),
              dataIndex: 'git_remote',
              ellipsis: true,
              render: (v) => (v ? <code>{v}</code> : <Text type="secondary">-</Text>)
            },
            {
              title: t('projects.description'),
              dataIndex: 'description',
              ellipsis: true
            },
            {
              title: t('projects.status'),
              dataIndex: 'status',
              width: 90,
              render: (v) => <Tag color={v === 'active' ? 'green' : 'default'}>{v}</Tag>
            },
            {
              title: t('projects.createdAt'),
              dataIndex: 'created_at',
              width: 170,
              render: (v) => formatDate(v)
            },
            {
              title: t('projects.actions'),
              width: 200,
              render: (_, r) => (
                <>
                  <Button type="link" icon={<EditOutlined />} onClick={() => onEdit(r)}>
                    {t('projects.edit')}
                  </Button>
                  <Popconfirm title={t('projects.confirmDelete')} onConfirm={() => onDelete(r.id)}>
                    <Button type="link" danger icon={<DeleteOutlined />}>
                      {t('projects.delete')}
                    </Button>
                  </Popconfirm>
                </>
              )
            }
          ]}
        />
      </Card>

      <Modal
        title={editing ? t('projects.editProject') : t('projects.newProject')}
        open={open}
        onCancel={() => setOpen(false)}
        onOk={onSubmit}
        destroyOnClose
        width={600}
      >
        <Form form={form} layout="vertical">
          <Form.Item name="name" label={t('projects.name')} rules={[{ required: true, message: t('projects.name') }]}>
            <Input placeholder={t('projects.namePlaceholder')} />
          </Form.Item>
          <Form.Item name="slug" label={t('projects.slug')} rules={[{ required: true, message: t('projects.slug') }]}>
            <Input placeholder={t('projects.slugPlaceholder')} />
          </Form.Item>
          <Form.Item name="description" label={t('projects.description')}>
            <TextArea rows={3} placeholder={t('projects.descriptionPlaceholder')} />
          </Form.Item>
          <Form.Item name="git_remote" label={t('projects.gitRemote')} tooltip={t('projects.gitRemoteHint')}>
            <Input placeholder={t('projects.gitRemotePlaceholder')} />
          </Form.Item>
          <Form.Item name="repo_name" label={t('projects.projectDir')} tooltip={t('projects.projectDirHint')}>
            <Input placeholder={t('projects.projectDirPlaceholder')} />
          </Form.Item>
          <Form.Item name="stack" label={t('projects.stack')}>
            <TextArea rows={2} placeholder={t('projects.stackPlaceholder')} />
          </Form.Item>
          <Form.Item name="structure" label={t('projects.structure')}>
            <TextArea rows={2} placeholder={t('projects.structurePlaceholder')} />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}

export default ProjectsPage
