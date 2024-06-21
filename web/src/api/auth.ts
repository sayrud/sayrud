import axios from 'axios';


export interface GitHubCallbackResp {
    token: string;
}

export function githubCallback(code: string) {
    return axios.get<GitHubCallbackResp, GitHubCallbackResp>('/auth/github/callback', {params: {code}});
}
