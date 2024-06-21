export default [
    {
        path: '/sign-in',
        component: () => import ('@/pages/auth/SignIn.vue'),
        name: 'SignIn',
    },
    {
        path: '/auth/github',
        component: () => import ('@/pages/auth/AuthGitHub.vue'),
        name: 'GitHubCallback',
    },
    {
        path: '/profile',
        component: () => import ('@/layouts/Console.vue'),
        children: [
            {
                path: '',
                component: () => import ('@/pages/auth/Profile.vue'),
                name: 'Profile',
                meta: {
                    auth: true,
                }
            }
        ]
    }
]
