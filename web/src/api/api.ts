import axios from "axios";

export type MethodType = 'GET' | 'POST' | 'PUT' | 'DELETE'

export type Kind = 'list' | 'view' | 'create' | 'update' | 'delete'

export interface Api {
    uid: string;
    kind: Kind;
    methods: MethodType[];
    path: string;
    queryParams: QueryParams;
    bodyParams: BodyParams;
    options: {
        datasets?: Dataset[];

        tableUID: string;
        filter?: string;
        fieldMapping?: Record<string, string>;
    };
    middlewares: Middleware[];
    datasets: Datasets;
    response: object;
}

export interface Middleware {
    type: string
    params: Record<string, string>
}

export type Validators = Validator[]

export interface Validator {
    expression: string;
    message: string;
}

export type ParamKind = 'query' | 'body'
export type ParamType = 'string' | 'number' | 'boolean'
export const ParamTypeLabels: Record<ParamType, string> = {
    'string': '字符串',
    'number': '数字',
    'boolean': '是/否',
}

export const ParamTypeOptions = [
    {label: '字符串', value: 'string'},
    {label: '数字', value: 'number'},
    {label: '是/否', value: 'boolean'},
]

export interface Param {
    no?: string;
    key: string;
    label: string;
    type: ParamType;
    required: boolean;
    customValidators: Validators;
}

export const ParamKindLabels: Record<string, string> = {
    'query': '查询参数',
    'body': '请求体',
}

export const ParamKindOptions = [
    {label: '查询参数', value: 'query'},
    {label: '请求体', value: 'body'},
]

export interface ParamMixin extends Param {
    kind: string;
}

export type QueryParams = QueryParam[]

export interface QueryParam extends Param {

}

export type  BodyParams = BodyParam[]

export interface BodyParam extends Param {

}

export type Datasets = Dataset[]

export interface Dataset {
    tableUID: string;
    fields: string[];
    filterExp: string;
    order: string[];
    fieldAlias: Record<string, string>
    limitExp: string;
    offsetExp: string;
}

export interface ListApisResp {
    apis: Api[];
    total: number;
}

export function listApis(projectUID: string, params: {
    page?: number,
    pageSize?: number
}) {
    return axios.get<ListApisResp, ListApisResp>(`/projects/${projectUID}/apis`, {params});
}

export function getApi(projectUID: string, apiUID: string) {
    return axios.get<Api, Api>(`/projects/${projectUID}/apis/${apiUID}`);
}

export interface CreateApiReq {
    kind: Kind;
    methods: string[];
    path: string;
    queryParams: QueryParams;
    bodyParams: BodyParams;
    filter?: string;
    fieldMapping?: Record<string, string>;
    datasets: Datasets;
    middlewares: Middleware[];
    response: string;
}

export function createApi(projectUID: string, data: CreateApiReq) {
    return axios.post<Api, Api>(`/projects/${projectUID}/apis`, data);
}

export interface UpdateApiReq {
    kind: Kind;
    methods: string[];
    path: string;
    queryParams: QueryParams;
    bodyParams: BodyParams;
    filter?: string;
    fieldMapping?: Record<string, string>;
    datasets: Datasets;
    middlewares: Middleware[];
    response: object;
}

export function updateApi(projectUID: string, apiUID: string, data: UpdateApiReq) {
    return axios.put<void, void>(`/projects/${projectUID}/apis/${apiUID}`, data);
}

export function deleteApi(projectUID: string, apiUID: string) {
    return axios.delete<void, void>(`/projects/${projectUID}/apis/${apiUID}`);
}
