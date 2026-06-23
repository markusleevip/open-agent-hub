import { useEffect, useState } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import {
  App,
  Card,
  Tabs,
  Descriptions,
  Table,
  Tag,
  Button,
  Typography,
  Space,
  Spin,
  Alert,
  Modal,
  Form,
  Input
} from 'antd'
import { ArrowLeftOutlined, EditOutlined } from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import { projectApi } from '@/api'
import RuleManager from '@/components/RuleManager'
import type { Project, SyncRecord } from '@/types'
import { formatDate, shortId } from '@/utils'

const { Title, Text, Paragraph } = Typography

const codeOrEmpty = (v: string | undefined, empty: string) =>
  v ? <code>{v}</code> : <Text type="secondary">{empty}</Text>

const ProjectDetailPage = () => {
  const { t } = useTranslation()
  const { id = '' } = useParams()
  const navigate = useNavigate()

  const [project, setProject] = useState<Project | null>(null)
  const [records, setRecords] = useState<SyncRecord[]>([])
  const [currentEtag, setCurrentEtag] = useState('')
  const [loading, setLoading] = useState(true)
  const [syncLoading, setSyncLoading] = useState(false)
  const [editOpen, setEditOpen] = useState(false)
  const [editLoading, setEditLoading] = useState(false)
  const [form] = Form.useForm()
  const { message } = App.useApp()

  useEffect(() => {
    projectApi
      .get(id)
      .then(setProject)
      .finally(() => setLoading(false))
  }, [id])

  const loadSync = async () => {
    setSyncLoading(true)
    try {
      const res = await projectApi.syncRecords(id)
      setRecords(res.records || [])
      setCurrentEtag(res.current_etag || '')
    } finally {
      setSyncLoading(false)
    }
  }

  const openEdit = () => {
    if (!project) return
    form.setFieldsValue({
      name: project.name,
      description: project.description || '',
      git_remote: project.git_remote || '',
      repo_name: project.repo_name || '',
      stack: project.stack || '',
      structure: project.structure || ''
    })
    setEditOpen(true)
  }

  const handleEdit = async () => {
    try {
      const values = await form.validateFields()
      setEditLoading(true)
      const updated = await projectApi.update(id, values)
      setProject(updated)
      setEditOpen(false)
      message.success(t('projects.detail.editSuccess'))
    } catch {
      // validation error, ignore
    } finally {
      setEditLoading(false)
    }
  }

  if (loading) {
    return (
      <div style={{ padding: 48, textAlign: 'center' }}>
        <Spin />
      </div>
    )
  }
  if (!project) {
    return (
      <div className="page-container">
        <Alert type="error" message="Project not found" showIcon />
      </div>
    )
  }

  const empty = t('projects.detail.empty')

  const overview = (
    <Card>
      <Descriptions
        title={t('projects.detail.identity')}
        column={1}
        bordered
        size="middle"
        styles={{ label: { width: 200 } }}
      >
        <Descriptions.Item label={t('projects.name')}>{project.name}</Descriptions.Item>
        <Descriptions.Item label={t('projects.slug')}>
          <code>{project.slug}</code>
        </Descriptions.Item>
        <Descriptions.Item label={t('projects.detail.projectId')}>
          <code>{project.id}</code>
        </Descriptions.Item>
        <Descriptions.Item label={t('projects.status')}>
          <Tag color={project.status === 'active' ? 'green' : 'default'}>{project.status}</Tag>
        </Descriptions.Item>
        <Descriptions.Item label={t('projects.description')}>
          {project.description || <Text type="secondary">{empty}</Text>}
        </Descriptions.Item>
        <Descriptions.Item label={t('projects.gitRemote')}>
          {codeOrEmpty(project.git_remote, empty)}
        </Descriptions.Item>
        <Descriptions.Item label={t('projects.projectDir')}>
          {codeOrEmpty(project.repo_name, empty)}
        </Descriptions.Item>
        <Descriptions.Item label={t('projects.detail.repoPath')}>
          {codeOrEmpty(project.repo_path, empty)}
        </Descriptions.Item>
        <Descriptions.Item label={t('projects.createdAt')}>
          {formatDate(project.created_at)}
        </Descriptions.Item>
      </Descriptions>

      <Text type="secondary" style={{ display: 'block', marginTop: 12 }}>
        {t('projects.detail.bindingHint')}
      </Text>

      {(project.stack || project.structure) && (
        <div style={{ marginTop: 24 }}>
          {project.stack && (
            <>
              <Title level={5}>{t('projects.detail.stack')}</Title>
              <Paragraph>
                <pre style={{ background: 'var(--code-bg, #f5f5f5)', padding: 12, borderRadius: 6 }}>
                  {project.stack}
                </pre>
              </Paragraph>
            </>
          )}
          {project.structure && (
            <>
              <Title level={5}>{t('projects.detail.structure')}</Title>
              <Paragraph>
                <pre style={{ background: 'var(--code-bg, #f5f5f5)', padding: 12, borderRadius: 6 }}>
                  {project.structure}
                </pre>
              </Paragraph>
            </>
          )}
        </div>
      )}

      <Alert
        style={{ marginTop: 16 }}
        type="info"
        showIcon
        message={t('projects.detail.aggregateHint')}
      />
    </Card>
  )

  const syncStatus = (
    <Card>
      <Space direction="vertical" size="small" style={{ width: '100%', marginBottom: 12 }}>
        <Text type="secondary">{t('projects.detail.syncDesc')}</Text>
        {currentEtag && (
          <Text type="secondary">
            {t('projects.detail.currentEtag')}: <code>{currentEtag.slice(0, 12)}</code>
          </Text>
        )}
      </Space>
      <Table<SyncRecord>
        rowKey="id"
        loading={syncLoading}
        dataSource={records}
        locale={{ emptyText: t('projects.detail.noSync') }}
        pagination={false}
        columns={[
          {
            title: t('projects.detail.colMember'),
            render: (_, r) => r.user_display_name || r.username || shortId(r.user_id)
          },
          {
            title: t('projects.detail.colClient'),
            dataIndex: 'client_name',
            render: (v, r) => <Tag color="blue">{v || r.client || 'unknown'}</Tag>
          },
          {
            title: t('projects.detail.colMachine'),
            dataIndex: 'repo_path',
            ellipsis: true,
            render: (v) => (v ? <code>{v}</code> : <Text type="secondary">-</Text>)
          },
          {
            title: t('projects.detail.colEtag'),
            dataIndex: 'etag',
            width: 130,
            render: (v) => <code>{v ? v.slice(0, 12) : '-'}</code>
          },
          {
            title: t('projects.detail.colSyncCount'),
            dataIndex: 'sync_count',
            width: 90
          },
          {
            title: t('projects.detail.colSyncedAt'),
            dataIndex: 'synced_at',
            width: 180,
            render: (v) => formatDate(v)
          },
          {
            title: t('projects.detail.colState'),
            width: 110,
            render: (_, r) =>
              r.stale ? (
                <Tag color="orange">{t('projects.detail.stale')}</Tag>
              ) : (
                <Tag color="green">{t('projects.detail.upToDate')}</Tag>
              )
          }
        ]}
      />
    </Card>
  )

  return (
    <div className="page-container">
      <Space align="center" style={{ marginBottom: 16 }}>
        <Button icon={<ArrowLeftOutlined />} onClick={() => navigate('/projects')}>
          {t('projects.detail.backToList')}
        </Button>
        <Title level={4} style={{ margin: 0 }}>
          {project.name}
        </Title>
        <code>{project.slug}</code>
        <Button icon={<EditOutlined />} onClick={openEdit}>
          {t('projects.detail.editBtn')}
        </Button>
      </Space>

      <Tabs
        defaultActiveKey="overview"
        destroyOnHidden
        onChange={(k) => {
          if (k === 'sync' && records.length === 0) loadSync()
        }}
        items={[
          { key: 'overview', label: t('projects.detail.tabOverview'), children: overview },
          {
            key: 'rules',
            label: t('projects.detail.tabRules'),
            children: (
              <RuleManager
                title={t('projects.detail.tabRules')}
                description={project.name}
                scope="project"
                fixedType="project_rule"
                withProject
                lockedProjectId={project.id}
              />
            )
          },
          { key: 'sync', label: t('projects.detail.tabSync'), children: syncStatus }
        ]}
      />

      <Modal
        title={t('projects.detail.editTitle')}
        open={editOpen}
        onCancel={() => setEditOpen(false)}
        onOk={handleEdit}
        confirmLoading={editLoading}
        destroyOnClose
      >
        <Form form={form} layout="vertical">
          <Form.Item name="name" label={t('projects.name')} rules={[{ required: true }]}>
            <Input placeholder={t('projects.namePlaceholder')} />
          </Form.Item>
          <Form.Item name="description" label={t('projects.description')}>
            <Input.TextArea rows={2} placeholder={t('projects.descriptionPlaceholder')} />
          </Form.Item>
          <Form.Item name="git_remote" label={t('projects.gitRemote')} tooltip={t('projects.gitRemoteHint')}>
            <Input placeholder={t('projects.gitRemotePlaceholder')} />
          </Form.Item>
          <Form.Item name="repo_name" label={t('projects.projectDir')} tooltip={t('projects.projectDirHint')}>
            <Input placeholder={t('projects.projectDirPlaceholder')} />
          </Form.Item>
          <Form.Item name="stack" label={t('projects.stack')}>
            <Input.TextArea rows={2} placeholder={t('projects.stackPlaceholder')} />
          </Form.Item>
          <Form.Item name="structure" label={t('projects.structure')}>
            <Input.TextArea rows={2} placeholder={t('projects.structurePlaceholder')} />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}

export default ProjectDetailPage
