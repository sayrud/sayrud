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
    }
]
