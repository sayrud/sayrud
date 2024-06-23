import axios from "axios";

export type MethodType = 'GET' | 'POST' | 'PUT' | 'DELETE'

export interface Api {
    uid: string;
    methods: MethodType[];
    path: string;
    queryParams: QueryParams;
    bodyParams: BodyParams;
    datasets: string[];
    response: object;
}

export type Validators = Validator[]

export interface Validator {
    expression: string;
    message: string;
}

export type ParamType = 'string' | 'number' | 'boolean'

export interface Params {
    key: string;
    label: string;
    type: ParamType;
    required: boolean;
    customValidators: Validators;
}

export type QueryParams = QueryParam[]

export interface QueryParam extends Params {

}

export type  BodyParams = BodyParam[]

export interface BodyParam extends Params {

}

export type Datasets = Dataset[]

export interface Dataset {
    tableUID: string;
    fields: string[];
    filterExp: string;
    order: string[];
    limit: string;
    offset: string;
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
    methods: string[];
    path: string;
    queryParams: QueryParams;
    bodyParams: BodyParams;
    datasets: Datasets;
    response: object;
}

export function createApi(projectUID: string, data: CreateApiReq) {
    return axios.post<Api, Api>(`/projects/${projectUID}/apis`, data);
}

export interface UpdateApiReq {
    methods: string[];
    path: string;
    queryParams: QueryParams;
    bodyParams: BodyParams;
    datasets: Datasets;
    response: object;
}

export function updateApi(projectUID: string, apiUID: string, data: UpdateApiReq) {
    return axios.put<void, void>(`/projects/${projectUID}/apis/${apiUID}`, data);
}

export function deleteApi(projectUID: string, apiUID: string) {
    return axios.delete<void, void>(`/projects/${projectUID}/apis/${apiUID}`);
}
