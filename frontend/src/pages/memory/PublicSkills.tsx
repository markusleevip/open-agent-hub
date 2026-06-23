import { useEffect, useMemo, useState } from 'react'
import {
  App,
  Alert,
  Button,
  Card,
  Form,
  Input,
  Modal,
  Popconfirm,
  Select,
  Space,
  Switch,
  Table,
  Tag,
  Typography
} from 'antd'
import {
  CheckCircleOutlined,
  DownloadOutlined,
  EditOutlined,
  EyeOutlined,
  PauseCircleOutlined,
  PlayCircleOutlined,
  PlusOutlined,
  StopOutlined,
  SyncOutlined
} from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import { projectApi, publicSkillApi, skillInstallApi } from '@/api'
import { useAuthStore } from '@/store/auth'
import type { Project, PublicSkillTemplate, SkillInstall } from '@/types'
import { formatDate, parseJSONArray, shortId } from '@/utils'

const { Title, Text, Paragraph } = Typography
const { TextArea } = Input

type InstalledFilter = 'all' | 'true' | 'false'
type TemplateStatus = 'draft' | 'active' | 'archived'

type TemplateFormValues = {
  slug: string
  name: string
  description?: string
  content: string
  category: string
  tags?: string
  risk_level: 'low' | 'medium' | 'high'
  status: TemplateStatus
}

const RISK_COLOR: Record<string, string> = {
  low: 'green',
  medium: 'gold',
  high: 'red'
}

const STATUS_COLOR: Record<string, string> = {
  draft: 'default',
  active: 'green',
  archived: 'default'
}

const INSTALL_COLOR: Record<string, string> = {
  active: 'green',
  disabled: 'gold',
  archived: 'default'
}

const tagsToText = (tags: string) => parseJSONArray(tags).join(', ')

const textToTags = (value?: string) =>
  (value || '')
    .split(',')
    .map((tag) => tag.trim())
    .filter(Boolean)

const installTagColor = (installs: SkillInstall[]) => {
  if (installs.some((install) => install.state === 'active')) {
    return INSTALL_COLOR.active
  }
  if (installs.some((install) => install.state === 'disabled')) {
    return INSTALL_COLOR.disabled
  }
  return INSTALL_COLOR.archived
}

const PublicSkills = () => {
  const { t } = useTranslation()
  const { message } = App.useApp()
  const role = useAuthStore((state) => state.role)
  const canManage = role === 'owner' || role === 'admin'

  const [data, setData] = useState<PublicSkillTemplate[]>([])
  const [projects, setProjects] = useState<Project[]>([])
  const [loading, setLoading] = useState(false)
  const [installOpen, setInstallOpen] = useState(false)
  const [templateOpen, setTemplateOpen] = useState(false)
  const [target, setTarget] = useState<PublicSkillTemplate | null>(null)
  const [editing, setEditing] = useState<PublicSkillTemplate | null>(null)
  const [detail, setDetail] = useState<PublicSkillTemplate | null>(null)
  const [category, setCategory] = useState<string>()
  const [status, setStatus] = useState<TemplateStatus | 'all'>('active')
  const [installed, setInstalled] = useState<InstalledFilter>('all')
  const [keyword, setKeyword] = useState('')
  const [installForm] = Form.useForm()
  const [templateForm] = Form.useForm<TemplateFormValues>()

  const categories = useMemo(() => {
    const set = new Set(data.map((item) => item.category).filter(Boolean))
    return Array.from(set).sort().map((value) => ({ value, label: value }))
  }, [data])

  const load = async () => {
    setLoading(true)
    try {
      const [skills, projectList] = await Promise.all([
        publicSkillApi.list({
          category,
          keyword: keyword.trim() || undefined,
          status,
          installed: installed === 'all' ? undefined : installed === 'true'
        }),
        projects.length === 0 ? projectApi.list() : Promise.resolve(projects)
      ])
      setData(skills)
      setProjects(projectList)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    load()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [category, status, installed])

  const openCreate = () => {
    setEditing(null)
    templateForm.resetFields()
    templateForm.setFieldsValue({
      risk_level: 'low',
      status: 'active',
      category: 'general'
    })
    setTemplateOpen(true)
  }

  const openEdit = (row: PublicSkillTemplate) => {
    setEditing(row)
    templateForm.resetFields()
    templateForm.setFieldsValue({
      slug: row.slug,
      name: row.name,
      description: row.description,
      content: row.content,
      category: row.category,
      tags: tagsToText(row.tags),
      risk_level: row.risk_level as TemplateFormValues['risk_level'],
      status: row.status as TemplateStatus
    })
    setTemplateOpen(true)
  }

  const submitTemplate = async () => {
    const values = await templateForm.validateFields()
    const payload = {
      slug: values.slug,
      name: values.name,
      description: values.description,
      content: values.content,
      category: values.category,
      tags: textToTags(values.tags),
      risk_level: values.risk_level,
      status: values.status
    }
    if (editing) {
      await publicSkillApi.update(editing.id, payload)
      message.success(t('memory.publicSkills.updated'))
    } else {
      await publicSkillApi.create(payload)
      message.success(t('memory.publicSkills.created'))
    }
    setTemplateOpen(false)
    setEditing(null)
    load()
  }

  const changeTemplateStatus = async (row: PublicSkillTemplate, nextStatus: TemplateStatus) => {
    await publicSkillApi.changeStatus(row.id, nextStatus)
    message.success(t('memory.publicSkills.statusUpdated'))
    load()
  }

  const occupiedInstalls = (row: PublicSkillTemplate) => row.installs?.filter((install) => install.state !== 'archived') || []
  const hasWorkspaceInstall = (row: PublicSkillTemplate) => occupiedInstalls(row).some((install) => !install.project_id)
  const installedProjectIDs = (row: PublicSkillTemplate) => new Set(occupiedInstalls(row).map((install) => install.project_id).filter(Boolean) as string[])
  const availableProjects = (row: PublicSkillTemplate) => {
    const occupied = installedProjectIDs(row)
    return projects.filter((project) => !occupied.has(project.id))
  }
  const hasInstallScopeAvailable = (row: PublicSkillTemplate) => !hasWorkspaceInstall(row) || availableProjects(row).length > 0
  const installedProjectCount = (row: PublicSkillTemplate) => installedProjectIDs(row).size
  const projectOptions = (row: PublicSkillTemplate) => {
    const occupied = installedProjectIDs(row)
    return projects.map((project) => {
      const isInstalled = occupied.has(project.id)
      return {
        value: project.id,
        label: isInstalled ? `${project.name} (${t('memory.publicSkills.alreadyInstalled')})` : project.name,
        disabled: isInstalled
      }
    })
  }

  const openInstall = (row: PublicSkillTemplate) => {
    setTarget(row)
    installForm.resetFields()
    const projectsAvailable = availableProjects(row)
    const defaultScope = hasWorkspaceInstall(row) ? 'project' : 'workspace'
    installForm.setFieldsValue({
      scope: defaultScope,
      project_id: defaultScope === 'project' ? projectsAvailable[0]?.id : undefined,
      pinned: false
    })
    setInstallOpen(true)
  }

  const submitInstall = async () => {
    const values = await installForm.validateFields()
    await skillInstallApi.create({
      template_id: target!.id,
      project_id: values.scope === 'project' ? values.project_id : undefined,
      pinned: values.pinned
    })
    message.success(t('memory.publicSkills.installed'))
    setInstallOpen(false)
    setTarget(null)
    load()
  }

  const changeInstallState = async (install: SkillInstall, state: 'active' | 'disabled' | 'archived') => {
    await skillInstallApi.changeState(install.id, state)
    message.success(t('memory.publicSkills.stateUpdated'))
    load()
  }

  const upgradeInstall = async (install: SkillInstall) => {
    await skillInstallApi.upgrade(install.id)
    message.success(t('memory.publicSkills.upgraded'))
    load()
  }

  const activeInstall = (row: PublicSkillTemplate) => row.installs?.find((install) => install.state === 'active')
  const disabledInstall = (row: PublicSkillTemplate) => row.installs?.find((install) => install.state === 'disabled')
  const firstInstall = (row: PublicSkillTemplate) => row.installs?.[0]

  return (
    <div className="page-container">
      <Card title={<Title level={4} style={{ margin: 0 }}>{t('memory.publicSkills.title')}</Title>}>
        <Text type="secondary" style={{ display: 'block', marginBottom: 16 }}>
          {t('memory.publicSkills.description')}
        </Text>
        <div
          style={{
            display: 'flex',
            flexWrap: 'wrap',
            gap: 12,
            justifyContent: 'space-between',
            marginBottom: 16
          }}
        >
          <Space wrap>
            <Input.Search
              allowClear
              placeholder={t('memory.publicSkills.keyword')}
              value={keyword}
              onChange={(event) => setKeyword(event.target.value)}
              onSearch={load}
              style={{ width: 240 }}
            />
            <Select
              allowClear
              placeholder={t('memory.publicSkills.category')}
              value={category}
              onChange={setCategory}
              options={categories}
              style={{ width: 160 }}
            />
            <Select
              value={status}
              onChange={setStatus}
              options={[
                { value: 'active', label: t('memory.publicSkills.status.active') },
                { value: 'draft', label: t('memory.publicSkills.status.draft') },
                { value: 'archived', label: t('memory.publicSkills.status.archived') },
                { value: 'all', label: t('memory.publicSkills.filters.all') }
              ]}
              style={{ width: 150 }}
            />
            <Select
              value={installed}
              onChange={setInstalled}
              options={[
                { value: 'all', label: t('memory.publicSkills.filters.all') },
                { value: 'true', label: t('memory.publicSkills.filters.installed') },
                { value: 'false', label: t('memory.publicSkills.filters.notInstalled') }
              ]}
              style={{ width: 150 }}
            />
            <Button onClick={load}>{t('common.refresh')}</Button>
          </Space>
          {canManage && (
            <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>
              {t('memory.publicSkills.newButton')}
            </Button>
          )}
        </div>
        <Table
          rowKey="id"
          loading={loading}
          dataSource={data}
          pagination={{ pageSize: 20 }}
          tableLayout="fixed"
          scroll={{ x: 1320 }}
          columns={[
            {
              title: t('memory.publicSkills.columns.name'),
              dataIndex: 'name',
              width: 330,
              render: (_, row) => (
                <Space direction="vertical" size={2} style={{ maxWidth: '100%' }}>
                  <Text strong>{row.name}</Text>
                  <Text type="secondary" ellipsis>
                    {row.description}
                  </Text>
                  <Space size={6} wrap>
                    <Tag>{row.slug}</Tag>
                    <code>{shortId(row.id)}</code>
                  </Space>
                </Space>
              )
            },
            {
              title: t('memory.publicSkills.columns.category'),
              dataIndex: 'category',
              width: 110,
              render: (value) => <Tag>{value}</Tag>
            },
            {
              title: t('memory.publicSkills.columns.risk'),
              dataIndex: 'risk_level',
              width: 80,
              render: (value) => <Tag color={RISK_COLOR[value] || 'default'}>{value}</Tag>
            },
            {
              title: t('memory.publicSkills.columns.version'),
              dataIndex: 'version',
              width: 70,
              render: (value) => `v${value}`
            },
            {
              title: t('memory.publicSkills.columns.status'),
              dataIndex: 'status',
              width: 90,
              render: (value) => <Tag color={STATUS_COLOR[value] || 'default'}>{t(`memory.publicSkills.status.${value}`)}</Tag>
            },
            {
              title: t('memory.publicSkills.columns.tags'),
              dataIndex: 'tags',
              width: 190,
              render: (value) => (
                <Space size={4} wrap>
                  {parseJSONArray(value).map((tag) => (
                    <Tag key={tag}>{tag}</Tag>
                  ))}
                </Space>
              )
            },
            {
              title: t('memory.publicSkills.columns.installed'),
              width: 150,
              render: (_, row) => {
                const installs = occupiedInstalls(row)
                if (installs.length === 0) {
                  return <Text type="secondary">-</Text>
                }
                const workspaceInstall = installs.find((install) => !install.project_id)
                const projectInstalls = installs.filter((install) => install.project_id)
                return (
                  <Space direction="vertical" size={2}>
                    {workspaceInstall && (
                      <Tag color={INSTALL_COLOR[workspaceInstall.state] || 'default'}>
                        {t('memory.publicSkills.workspaceScoped')} · {workspaceInstall.state}
                      </Tag>
                    )}
                    {projectInstalls.length > 0 && (
                      <Tag color={installTagColor(projectInstalls)}>
                        {t('memory.publicSkills.projectInstallSummary', { installed: projectInstalls.length, total: projects.length })}
                      </Tag>
                    )}
                  </Space>
                )
              }
            },
            {
              title: t('memory.publicSkills.columns.updated'),
              dataIndex: 'updated_at',
              width: 140,
              render: (value) => formatDate(value)
            },
            {
              title: t('memory.publicSkills.columns.actions'),
              width: 280,
              fixed: 'right',
              render: (_, row) => {
                const active = activeInstall(row)
                const disabled = disabledInstall(row)
                const install = firstInstall(row)
                return (
                  <Space size="small" wrap>
                    <Button size="small" icon={<EyeOutlined />} onClick={() => setDetail(row)}>
                      {t('memory.publicSkills.view')}
                    </Button>
                    {canManage && (
                      <Button size="small" icon={<EditOutlined />} onClick={() => openEdit(row)}>
                        {t('memory.publicSkills.edit')}
                      </Button>
                    )}
                    {canManage && row.status !== 'active' && (
                      <Button size="small" icon={<CheckCircleOutlined />} onClick={() => changeTemplateStatus(row, 'active')}>
                        {t('memory.publicSkills.activate')}
                      </Button>
                    )}
                    {canManage && row.status !== 'archived' && (
                      <Popconfirm title={t('memory.publicSkills.confirmArchiveTemplate')} onConfirm={() => changeTemplateStatus(row, 'archived')}>
                        <Button size="small" danger icon={<StopOutlined />}>
                          {t('memory.publicSkills.archiveTemplate')}
                        </Button>
                      </Popconfirm>
                    )}
                    {row.status === 'active' && (
                      <Button size="small" type="primary" icon={<DownloadOutlined />} onClick={() => openInstall(row)} disabled={row.risk_level === 'high'}>
                        {t('memory.publicSkills.install')}
                      </Button>
                    )}
                    {active && (
                      <Button size="small" icon={<PauseCircleOutlined />} onClick={() => changeInstallState(active, 'disabled')}>
                        {t('memory.publicSkills.disable')}
                      </Button>
                    )}
                    {disabled && (
                      <Button size="small" icon={<PlayCircleOutlined />} onClick={() => changeInstallState(disabled, 'active')}>
                        {t('memory.publicSkills.enable')}
                      </Button>
                    )}
                    {install && install.installed_version < row.version && (
                      <Button size="small" icon={<SyncOutlined />} onClick={() => upgradeInstall(install)}>
                        {t('memory.publicSkills.upgrade')}
                      </Button>
                    )}
                    {install && install.state !== 'archived' && (
                      <Popconfirm title={t('memory.publicSkills.confirmArchive')} onConfirm={() => changeInstallState(install, 'archived')}>
                        <Button size="small" danger icon={<StopOutlined />}>
                          {t('memory.publicSkills.archive')}
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
        title={editing ? `${t('memory.publicSkills.edit')} - ${editing.name}` : t('memory.publicSkills.newButton')}
        open={templateOpen}
        onCancel={() => setTemplateOpen(false)}
        onOk={submitTemplate}
        width={820}
        forceRender
        destroyOnHidden
      >
        <Form form={templateForm} layout="vertical">
          <Space style={{ width: '100%' }} size="middle" align="start">
            <Form.Item name="slug" label="Slug" rules={[{ required: true }, { pattern: /^[a-z0-9]+(?:-[a-z0-9]+)*$/, message: t('memory.publicSkills.form.slugRule') }]} style={{ width: 260 }}>
              <Input placeholder="api-review" />
            </Form.Item>
            <Form.Item name="name" label={t('memory.publicSkills.form.name')} rules={[{ required: true }]} style={{ width: 260 }}>
              <Input placeholder={t('memory.publicSkills.form.namePlaceholder')} />
            </Form.Item>
            <Form.Item name="category" label={t('memory.publicSkills.form.category')} rules={[{ required: true }]} style={{ width: 180 }}>
              <Input placeholder="backend" />
            </Form.Item>
          </Space>
          <Form.Item name="description" label={t('memory.publicSkills.form.description')}>
            <Input placeholder={t('memory.publicSkills.form.descriptionPlaceholder')} />
          </Form.Item>
          <Space style={{ width: '100%' }} size="middle" align="start">
            <Form.Item name="risk_level" label={t('memory.publicSkills.form.risk')} rules={[{ required: true }]} style={{ width: 180 }}>
              <Select
                options={[
                  { value: 'low', label: 'low' },
                  { value: 'medium', label: 'medium' },
                  { value: 'high', label: 'high' }
                ]}
              />
            </Form.Item>
            <Form.Item name="status" label={t('memory.publicSkills.form.status')} rules={[{ required: true }]} style={{ width: 180 }}>
              <Select
                options={[
                  { value: 'draft', label: t('memory.publicSkills.status.draft') },
                  { value: 'active', label: t('memory.publicSkills.status.active') },
                  { value: 'archived', label: t('memory.publicSkills.status.archived') }
                ]}
              />
            </Form.Item>
            <Form.Item name="tags" label={t('memory.publicSkills.form.tags')} style={{ flex: 1 }}>
              <Input placeholder={t('memory.publicSkills.form.tagsPlaceholder')} />
            </Form.Item>
          </Space>
          <Form.Item name="content" label={t('memory.publicSkills.form.content')} rules={[{ required: true }]}>
            <TextArea rows={12} placeholder={t('memory.publicSkills.form.contentPlaceholder')} />
          </Form.Item>
          {editing && <Text type="secondary">{t('memory.publicSkills.form.versionHint')}</Text>}
        </Form>
      </Modal>

      <Modal
        title={target ? `${t('memory.publicSkills.install')} - ${target.name}` : t('memory.publicSkills.install')}
        open={installOpen}
        onCancel={() => setInstallOpen(false)}
        onOk={submitInstall}
        okButtonProps={{ disabled: target ? !hasInstallScopeAvailable(target) : false }}
        forceRender
        destroyOnHidden
      >
        <Form form={installForm} layout="vertical">
          {target && !hasInstallScopeAvailable(target) && (
            <Alert type="info" showIcon message={t('memory.publicSkills.allScopesInstalled')} style={{ marginBottom: 16 }} />
          )}
          <Form.Item name="scope" label={t('memory.publicSkills.form.scope')} rules={[{ required: true }]}>
            <Select
              onChange={(value) => {
                if (!target) {
                  return
                }
                if (value === 'project') {
                  installForm.setFieldsValue({ project_id: availableProjects(target)[0]?.id })
                } else {
                  installForm.setFieldsValue({ project_id: undefined })
                }
              }}
              options={[
                {
                  value: 'workspace',
                  label: target && hasWorkspaceInstall(target) ? `${t('memory.publicSkills.workspaceScoped')} (${t('memory.publicSkills.alreadyInstalled')})` : t('memory.publicSkills.workspaceScoped'),
                  disabled: target ? hasWorkspaceInstall(target) : false
                },
                {
                  value: 'project',
                  label: t('memory.publicSkills.projectScoped'),
                  disabled: target ? availableProjects(target).length === 0 : false
                }
              ]}
            />
          </Form.Item>
          <Form.Item noStyle shouldUpdate={(prev, cur) => prev.scope !== cur.scope}>
            {({ getFieldValue }) =>
              getFieldValue('scope') === 'project' ? (
                <Form.Item name="project_id" label={t('memory.publicSkills.form.project')} rules={[{ required: true }]}>
                  <Select options={target ? projectOptions(target) : projects.map((project) => ({ value: project.id, label: project.name }))} />
                </Form.Item>
              ) : null
            }
          </Form.Item>
          <Form.Item name="pinned" label={t('memory.publicSkills.form.pinned')} valuePropName="checked">
            <Switch />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        title={detail?.name}
        open={!!detail}
        onCancel={() => setDetail(null)}
        footer={null}
        width={760}
        destroyOnHidden
      >
        {detail && (
          <Space direction="vertical" style={{ width: '100%' }} size="middle">
            <Paragraph type="secondary">{detail.description}</Paragraph>
            <Space wrap>
              <Tag>{detail.category}</Tag>
              <Tag color={RISK_COLOR[detail.risk_level] || 'default'}>{detail.risk_level}</Tag>
              <Tag color={STATUS_COLOR[detail.status] || 'default'}>{t(`memory.publicSkills.status.${detail.status}`)}</Tag>
              <Tag>v{detail.version}</Tag>
              {parseJSONArray(detail.tags).map((tag) => (
                <Tag key={tag}>{tag}</Tag>
              ))}
            </Space>
            <TextArea value={detail.content} rows={16} readOnly />
          </Space>
        )}
      </Modal>
    </div>
  )
}

export default PublicSkills
