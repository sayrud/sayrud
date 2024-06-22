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
        path: 'tables/:tableUID/records',
        component: () => import ('@/pages/schemaless/Records.vue'),
        name: 'SchemalessTableRecords',
        meta: {
            auth: true,
        },
    },
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
