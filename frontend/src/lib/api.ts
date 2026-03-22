import { GraphQLClient, gql } from "graphql-request";

export type NewsItem = {
  id: string;
  title: string;
  url: string;
  source: string;
  publishedAt: string;
  summary: string;
};

const endpoint = import.meta.env.VITE_API_URL
  ? `${import.meta.env.VITE_API_URL}/graphql`
  : "http://localhost:8080/graphql";

const client = new GraphQLClient(endpoint);

export async function fetchNews(limit = 30): Promise<NewsItem[]> {
  const query = gql`
    query News($limit: Int) {
      news(limit: $limit) {
        id
        title
        url
        source
        publishedAt
        summary
      }
    }
  `;
  const data = await client.request<{ news: NewsItem[] }>(query, { limit });
  return data.news;
}

export async function refresh(): Promise<boolean> {
  const mutation = gql`
    mutation {
      refresh
    }
  `;
  const data = await client.request<{ refresh: boolean }>(mutation);
  return data.refresh;
}

export function eventsURL(): string {
  if (import.meta.env.VITE_API_URL) return `${import.meta.env.VITE_API_URL}/events`;
  return "http://localhost:8080/events";
}

