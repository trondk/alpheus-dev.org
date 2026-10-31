# The component as an image. The stubs for component.proto are generated
# here, so the folder holds no generated code. Pin the base images by
# digest for a component you run; the StreamRun pins the component's image
# by digest (GO-26).
FROM docker.io/library/python:3.13-slim AS build
WORKDIR /src
COPY requirements.txt .
RUN pip install --no-cache-dir --root-user-action=ignore --target /deps -r requirements.txt \
 && pip install --no-cache-dir --root-user-action=ignore grpcio-tools==1.71.0
COPY alpheus/ alpheus/
RUN python -m grpc_tools.protoc -I. --python_out=. --grpc_python_out=. alpheus/component.proto

FROM docker.io/library/python:3.13-slim
COPY --from=build /deps /deps
COPY --from=build /src/alpheus /app/alpheus
COPY component.py /app/
ENV PYTHONPATH=/deps:/app PYTHONUNBUFFERED=1 PYTHONDONTWRITEBYTECODE=1
# The pod runs as uid 65532 with a read-only root file system.
USER 65532:65532
ENTRYPOINT ["python", "/app/component.py"]
