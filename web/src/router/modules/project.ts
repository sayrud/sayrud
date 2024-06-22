const schemaless = [
    {
        path: 'tables',
        component: () => import ('@/pages/schemaless/Tables.vue'),
        name: 'SchemalessTable',
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
                path: ':uid',
                component: () => import ('@/pages/project/ProjectView.vue'),
                name: 'ProjectView',
                meta: {
                    auth: true,
                },

                children: schemaless,
            }
        ]
    }
]
