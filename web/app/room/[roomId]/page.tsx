import RoomEditor from "@/components/RoomEditor";

export default async function RoomPage({
  params,
}: {
  params: Promise<{ roomId: string }>;
}) {
  const { roomId } = await params;
  return <RoomEditor roomId={roomId} />;
}
