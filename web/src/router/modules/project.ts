const schemaless = [
    {
        path: 'tables',
        component: () => import ('@/pages/schemaless/Tables.vue'),
        name: 'SchemalessTable',
        meta: {
            auth: true,
        },
    },
    {
        path: 'tables/:tableUID',
        component: () => import ('@/pages/schemaless/TableSettings.vue'),
        name: 'SchemalessTableSettings',
        meta: {
            auth: true,
        },
    },
    {
        path: 'records',
        component: () => import ('@/pages/schemaless/Records.vue'),
        name: 'SchemalessRecord',
        meta: {
            auth: true,
        },
    },
    // API
    {
        path: 'apis',
        component: () => import ('@/pages/schemaless/Apis.vue'),
        name: 'SchemalessApi',
        meta: {
            auth: true,
        },
    },
    {
        path: 'apis/create',
        component: () => import ('@/pages/schemaless/ApiSettings.vue'),
        name: 'SchemalessApiCreate',
        meta: {
            auth: true,
        },
    },
    {
        path: 'apis/:apiUID',
        component: () => import ('@/pages/schemaless/ApiSettings.vue'),
        name: 'SchemalessApiSettings',
        meta: {
            auth: true,
        },
    },
    // VIEWS
    {
        path: 'views',
        component: () => import ('@/pages/schemaless/Views.vue'),
        name: 'SchemalessView',
        meta: {
            auth: true,
        },
    },
    {
        path: 'view/create',
        component: () => import ('@/pages/schemaless/ViewSettings.vue'),
        name: 'SchemalessViewCreate',
        meta: {
            auth: true,
        },
    },
    {
        path: 'views/:viewUID/settings',
        component: () => import ('@/pages/schemaless/ViewSettings.vue'),
        name: 'SchemalessViewSettings',
        meta: {
            auth: true,
        },
    },
    {
        path: 'views/:viewUID',
        component: () => import ('@/pages/schemaless/TableView.vue'),
        name: 'SchemalessTableView',
        meta: {
            auth: true,
        },
    }
]

export default [
    {
        path: '/projects',
        component: () => import ('@/layouts/Console.vue'),

        children: [
            {
                path: '',
                component: () => import ('@/pages/project/Projects.vue'),
                name: 'Projects',
                meta: {
                    auth: true,
                }
            },
            {
                path: 'create',
                component: () => import ('@/pages/project/ProjectCreate.vue'),
                name: 'ProjectCreate',
                meta: {
                    auth: true,
                }
            },
            {
                path: ':uid/settings',
                component: () => import ('@/pages/project/ProjectSettings.vue'),
                name: 'ProjectSettings',
                meta: {
                    auth: true,
                },
            },
            {
                path: ':uid',
                component: () => import ('@/layouts/Project.vue'),
                meta: {
                    auth: true,
                },

                children: schemaless,
            }
        ]
    }
]
