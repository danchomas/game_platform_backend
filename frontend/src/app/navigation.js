import { Sparkles, LayoutGrid, ChartNoAxesCombined } from 'lucide-react';

export const NAVIGATION = [
  {
    id: 'studio',
    title: 'Студия игр',
    icon: Sparkles,
  },
  {
    id: 'gallery',
    title: 'Мои игры',
    icon: LayoutGrid,
  },
  {
    id: 'analytics',
    title: 'Аналитика',
    icon: ChartNoAxesCombined,
  },
];

export const PAGE_NAMES = Object.fromEntries(NAVIGATION.map((item) => [item.id, item.title]));
